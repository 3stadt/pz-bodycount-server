package main

import (
	"bufio"
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"github.com/fsnotify/fsnotify"
	"github.com/gorilla/websocket"
	"gopkg.in/yaml.v3"
	"html/template"
	"log"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type configData struct {
	ListenAddress string `yaml:"Listen_Address"`
	Template      string `yaml:"Template_File"`
	ModDataDir    string `yaml:"PZ_Mod_Data_Dir"`
}

type WeaponType struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

type WeaponCategory struct {
	Name     string `json:"name"`
	Quantity int    `json:"quantity"`
}

type pzStats struct {
	Total      int              `json:"total"`
	Categories []WeaponCategory `json:"categories"`
	Types      []WeaponType     `json:"types"`
}

var stats = pzStats{
	Total:      0,
	Categories: []WeaponCategory{},
	Types:      []WeaponType{},
}

var pzDir string

//go:embed template.gohtml
var statsTemplate string

var statsUpdated chan struct{} // keep channel minimal, we need only notification
var updateFromFile chan string

var clients map[*websocket.Conn]struct{}
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

const (
	CatFile   = "mod_bodycount_categories.json"
	TypeFile  = "mod_bodycount_types.json"
	TotalFile = "mod_bodycount_total.txt"
)

func main() {
	var conf configData
	statsUpdated = make(chan struct{}, 100)
	updateFromFile = make(chan string, 100)
	clients = map[*websocket.Conn]struct{}{}

	// Read config from file
	data, err := os.ReadFile("config.txt")
	if err != nil {
		waitForInputThenExit(err.Error())
	}
	if err := yaml.Unmarshal(data, &conf); err != nil {
		waitForInputThenExit(err.Error())
	}
	if conf.ListenAddress == "" {
		waitForInputThenExit("The config file is invalid. Please set Listen_Address according to the documentation.")
	}

	pzDir = conf.ModDataDir
	if pzDir == "" {
		usr, err := user.Current()
		if err != nil {
			waitForInputThenExit(err.Error())
		}
		pzDir = usr.HomeDir + "\\Zomboid\\Lua"
	}

	updateStatsFromFiles()

	if conf.Template != "" {
		tpl, err := os.ReadFile(conf.Template)
		if err != nil {
			waitForInputThenExit(err.Error())
		}
		statsTemplate = string(tpl)
	}

	go func() {
		for {
			select {
			case <-updateFromFile:
				updateStatsFromFiles()
				statsUpdated <- struct{}{}
			}
		}
	}()
	go watchPzDir(pzDir)

	httpServerExitDone := &sync.WaitGroup{}
	httpServerExitDone.Add(1)
	srv := conf.startHttpServer(httpServerExitDone)

	a := app.New()
	w := a.NewWindow("PZ BodyCount Server")
	w.Resize(fyne.Size{
		Width:  300,
		Height: 200,
	})
	content := container.New(layout.NewPaddedLayout(), container.New(layout.NewVBoxLayout(),
		widget.NewLabel("Project Zomboid OBS stat server is running."),
		layout.NewSpacer(),
		widget.NewLabel(fmt.Sprintf("The URL for your browser source is:\nhttp://%s", conf.ListenAddress)),
		widget.NewButtonWithIcon("Copy address to clipboard", theme.ContentCopyIcon(), func() {
			w.Clipboard().SetContent(fmt.Sprintf("http://%s", conf.ListenAddress))
		}),
	))
	w.SetContent(content)
	w.ShowAndRun()
	if err := srv.Shutdown(context.TODO()); err != nil {
		panic(err) // failure/timeout shutting down the server gracefully
	}

	// wait for goroutine started in startHttpServer() to stop
	httpServerExitDone.Wait()
}

func wsReader(ws *websocket.Conn) {
	defer func() {
		delete(clients, ws)
		ws.Close()
	}()
	for {
		mt, _, err := ws.ReadMessage()
		if err != nil || mt == websocket.CloseMessage {
			break // Exit the loop if the client tries to close the connection or the connection with the interrupted client
		}
	}
}

func wsWriter() {
	for {
		select {
		case <-statsUpdated:
			for conn := range clients {
				conn.WriteJSON(stats.Categories)
			}
		}
	}
}

func (conf *configData) startHttpServer(wg *sync.WaitGroup) *http.Server {
	srv := &http.Server{Addr: conf.ListenAddress}

	t := template.Must(template.New("stats").Parse(statsTemplate))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var data bytes.Buffer
		enc := json.NewEncoder(&data)
		enc.SetEscapeHTML(false)
		err := enc.Encode(stats.Categories)
		if err != nil {
			log.Fatalf("startHttpServer(), Encode(): %v", err)
		}

		err = t.Execute(w, struct {
			Host string
			Data template.JS
		}{conf.ListenAddress, template.JS(data.String())})
		if err != nil {
			log.Fatalf("startHttpServer(), Execute(): %v", err)
		}
	})

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			if _, ok := err.(websocket.HandshakeError); !ok {
				log.Println(err)
			}
			return
		}
		clients[ws] = struct{}{}
		go wsWriter()
		wsReader(ws)
	})

	go func() {
		defer wg.Done() // let main know we are done cleaning up

		// always returns error. ErrServerClosed on graceful close
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			// unexpected error. port in use?
			log.Fatalf("startHttpServer(), ListenAndServe(): %v", err)
		}
	}()

	// returning reference so caller can call Shutdown()
	return srv
}

func updateStatsFromFiles() {

	jsonData, err := readStatsFile(TypeFile)
	if err != nil {
		log.Printf("readStatsFile(TypeFile): %v", err)
		return
	}
	stats.Types = []WeaponType{}
	err = json.Unmarshal(jsonData, &stats.Types)
	if err != nil {
		log.Printf("json.Unmarshal(jsonData, &stats.Types): %v", err)
		return
	}

	jsonData, err = readStatsFile(CatFile)
	if err != nil {
		log.Printf("readStatsFile(CatFile): %v", err)
		return
	}
	stats.Categories = []WeaponCategory{}
	err = json.Unmarshal(jsonData, &stats.Categories)
	if err != nil {
		log.Printf("json.Unmarshal(jsonData, &stats.Categories): %v", err)
		return
	}

	totalData, err := readStatsFile(TotalFile)
	if err != nil {
		log.Printf("readStatsFile(TotalFile): %v", err)
		return
	}
	total, err := strconv.Atoi(string(totalData))
	if err != nil {
		log.Printf("strconv.Atoi(string(totalData)): %v", err)
		return
	}
	stats.Total = total
}

func readStatsFile(filename string) ([]byte, error) {
	data, err := os.ReadFile(pzDir + "\\" + filename)
	if err != nil {
		log.Printf("os.ReadFile() for %s with data %q: %v", pzDir+"\\"+filename, data, err)
		return data, err
	}

	tryCount := 0
	for string(data) == "" && tryCount <= 9 {
		data, err = os.ReadFile(pzDir + "\\" + filename)
		if err != nil {
			log.Printf("os.ReadFile() for %s with data %q: %v", pzDir+"\\"+filename, data, err)
			return data, err
		}
		tryCount++
		time.Sleep(time.Millisecond * 100)
	}
	return data, err
}

func watchPzDir(dir string) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event := <-watcher.Events:
				if event.Op&fsnotify.Write == fsnotify.Write {
					updateFromFile <- filepath.Base(event.Name)
				}
			case err := <-watcher.Errors:
				waitForInputThenExit(err.Error())
			}
		}
	}()

	err = watcher.Add(dir)
	if err != nil {
		waitForInputThenExit(err.Error())
	}
	<-done
}

func waitForInputThenExit(message string) {
	fmt.Print("\n\nERROR: " + message + "\n\n")
	fmt.Print("The server FAILED and is NOT RUNNING!\n")
	fmt.Print("Press any key to exit...")
	input := bufio.NewScanner(os.Stdin)
	input.Scan()
	os.Exit(0)
}
