package main

import (
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
	"gopkg.in/yaml.v3"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"time"
)

var stats = pzStats{
	Total:      0,
	Categories: []WeaponCategory{},
	Types:      []WeaponType{},
}

var pzDir string

//go:embed template.gohtml
var statsTemplate string

const (
	CatFile   = "mod_bodycount_categories.json"
	TypeFile  = "mod_bodycount_types.json"
	TotalFile = "mod_bodycount_total.txt"
)

var fLog *log.Logger

func main() {

	// open file and create if non-existent, also emptying it
	file, err := os.OpenFile("pz-bodycount-server-log.txt", os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	fLog = log.New(file, "", log.LstdFlags)

	var conf configData
	// Read config from file
	data, err := os.ReadFile("config.txt")
	if err != nil {
		fLog.Fatal(err)
	}
	if err := yaml.Unmarshal(data, &conf); err != nil {
		fLog.Fatal(err)
	}
	if conf.ListenAddress == "" {
		fLog.Fatal("The config file is invalid. Please set Listen_Address according to the documentation.")
	}
	if strings.HasPrefix(conf.ListenAddress, ":") {
		conf.ListenAddress = fmt.Sprintf("%s%s", GetOutboundIP().String(), conf.ListenAddress)
	}

	pzDir = conf.ModDataDir
	if pzDir == "" {
		usr, err := user.Current()
		if err != nil {
			fLog.Fatal(err)
		}
		pzDir = usr.HomeDir + "\\Zomboid\\Lua"
	}

	if conf.Template != "" {
		tpl, err := os.ReadFile(conf.Template)
		if err != nil {
			fLog.Fatal(err)
		}
		statsTemplate = string(tpl)
	}

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

func (conf *configData) startHttpServer(wg *sync.WaitGroup) *http.Server {
	srv := &http.Server{Addr: conf.ListenAddress}

	t := template.Must(template.New("stats").Parse(statsTemplate))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		updateStatsFromFiles()
		var data bytes.Buffer
		enc := json.NewEncoder(&data)
		enc.SetEscapeHTML(false)
		err := enc.Encode(stats)
		if err != nil {
			fLog.Fatalf("startHttpServer(), Encode(): %v", err)
		}

		err = t.Execute(w, struct {
			Host string
			Data template.JS
		}{conf.ListenAddress, template.JS(
			strings.TrimSpace(data.String()),
		)})
		if err != nil {
			fLog.Fatalf("startHttpServer(), Execute(): %v\n", err)
		}
	})

	go func() {
		defer wg.Done() // let main know we are done cleaning up

		// always returns error. ErrServerClosed on graceful close
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			// unexpected error. port in use?
			fLog.Fatalf("startHttpServer(), ListenAndServe(): %v\n", err)
		}
	}()

	// returning reference so caller can call Shutdown()
	return srv
}

func updateStatsFromFiles() {

	jsonData, err := readStatsFile(TypeFile)
	if err != nil {
		return
	}
	stats.Types = []WeaponType{}
	err = json.Unmarshal(jsonData, &stats.Types)
	if err != nil {
		fLog.Printf("json.Unmarshal(jsonData, &stats.Types): %v", err)
		return
	}

	jsonData, err = readStatsFile(CatFile)
	if err != nil {
		return
	}
	stats.Categories = []WeaponCategory{}
	err = json.Unmarshal(jsonData, &stats.Categories)
	if err != nil {
		fLog.Printf("json.Unmarshal(jsonData, &stats.Categories): %v", err)
		return
	}

	totalData, err := readStatsFile(TotalFile)
	if err != nil {
		return
	}
	total, err := strconv.Atoi(string(totalData))
	if err != nil {
		fLog.Printf("strconv.Atoi(string(totalData)): %v", err)
		return
	}
	stats.Total = total
}

func readStatsFile(filename string) ([]byte, error) {
	data, err := os.ReadFile(pzDir + "\\" + filename)
	if err != nil {
		fLog.Printf("os.ReadFile() for %s with data %q: %v", pzDir+"\\"+filename, data, err)
		return data, err
	}

	tryCount := 0
	for string(data) == "" && tryCount <= 9 {
		data, err = os.ReadFile(pzDir + "\\" + filename)
		if err != nil {
			fLog.Printf("os.ReadFile() for %s with data %q: %v", pzDir+"\\"+filename, data, err)
			return data, err
		}
		tryCount++
		time.Sleep(time.Millisecond * 100)
	}
	return data, err
}

// GetOutboundIP
// Get the IP of the interface used for outbound traffic, taken from SO.
// See https://stackoverflow.com/a/37382208
func GetOutboundIP() net.IP {
	conn, err := net.Dial("udp", "8.8.8.8:80") // Doesn't actually connect bc udp is used
	if err != nil {
		fLog.Fatal(err)
	}
	defer func(conn net.Conn) {
		err = conn.Close()
		if err != nil {
			fLog.Fatal(err)
		}
	}(conn)

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP
}
