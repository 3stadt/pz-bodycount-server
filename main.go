package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"os/user"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"gopkg.in/yaml.v3"
)

var stats = pzStats{
	Total:      0,
	Categories: []WeaponData{},
	Types:      []WeaponData{},
	ChartData:  "",
}

var pzDir string

//go:embed template.gohtml
var statsTemplate string

//go:embed template_chart.gohtml
var chartTemplate string

//go:embed chart.min.js
var chartJs string

const (
	ChartDataFile = "mod_bodycount_per_day_data.txt"
	CatFile       = "mod_bodycount_categories.json"
	TypeFile      = "mod_bodycount_types.json"
	TotalFile     = "mod_bodycount_total.txt"
)

type pzTime struct {
	time.Time
}

func (pzt *pzTime) UnmarshalJSON(b []byte) error {
	s := string(b)
	s = strings.Trim(s, "\"")
	t, err := time.Parse("2.1. 2006", s)
	if err != nil {
		return errors.New(fmt.Sprintf("Trying to parse %q: %s", string(b), err))
	}
	pzt.Time = t
	return nil
}

func (pzt *pzTime) MarshalJSON() ([]byte, error) {
	return []byte(`"` + pzt.Time.Format("2.1. 2006") + `"`), nil
}

type chartData struct {
	XVal pzTime `json:"x"`
	YVal int    `json:"y"`
}

type tplData struct {
	tpl  *template.Template
	conf *configData
}

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
	if conf.FontColor == "" {
		conf.FontColor = "grey"
	}
	if conf.ChartFontFamily == "" {
		conf.ChartFontFamily = "'Helvetica Neue', 'Helvetica', 'Arial', sans-serif"
	}
	if conf.ChartFontSize == "" {
		conf.ChartFontSize = "12"
	}

	pzDir = conf.ModDataDir
	if pzDir == "" {
		usr, err := user.Current()
		if err != nil {
			fLog.Fatal(err)
		}
		pzDir = usr.HomeDir + "\\Zomboid\\Lua"
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

	tplDataStats := tplData{
		tpl:  template.Must(template.New("stats").Parse(statsTemplate)),
		conf: conf,
	}

	tplDataChart := tplData{
		tpl:  template.Must(template.New("chart").Parse(chartTemplate)),
		conf: conf,
	}

	http.HandleFunc("/", tplDataStats.HandleIndex)
	http.HandleFunc("/chart", tplDataChart.HandleChart)

	http.HandleFunc("/chart.min.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		_, err := w.Write([]byte(chartJs))
		if err != nil {
			fLog.Printf("startHttpServer(), w.Write([]byte(chartJs)): %v\n", err)
		}
	})

	go func() {
		defer wg.Done() // let main know we are done cleaning up

		// always returns error. ErrServerClosed on graceful close
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			// unexpected error. port in use?
			fLog.Printf("startHttpServer(), ListenAndServe(): %v\n", err)
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
	stats.Types = []WeaponData{}
	err = json.Unmarshal(jsonData, &stats.Types)
	if err != nil {
		fLog.Printf("json.Unmarshal(jsonData, &stats.Types): %v", err)
		return
	}

	jsonData, err = readStatsFile(CatFile)
	if err != nil {
		return
	}
	stats.Categories = []WeaponData{}
	err = json.Unmarshal(jsonData, &stats.Categories)
	if err != nil {
		fLog.Printf("json.Unmarshal(jsonData, &stats.Categories): %v", err)
		return
	}

	chartDataBytes, err := readStatsFile(ChartDataFile)
	if err != nil {
		return
	}
	stats.ChartData = string(chartDataBytes)

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

func (td *tplData) HandleChart(w http.ResponseWriter, r *http.Request) {
	updateStatsFromFiles()

	cd := []chartData{}
	err := json.Unmarshal([]byte(stats.ChartData), &cd)
	if err != nil {
		fLog.Printf("HandleChart(), json.Unmarshal([]byte(stats.ChartData), &cd): %v\n", err)
		w.Write([]byte(fmt.Sprintf("HandleChart(), json.Unmarshal([]byte(stats.ChartData), &cd): %v\n", err)))
		return
	}

	sort.SliceStable(cd, func(i, j int) bool {
		return cd[i].XVal.Time.Before(cd[j].XVal.Time)
	})

	limitDays := r.URL.Query().Get("limitDays")
	if limitDays != "" {
		limit, err := strconv.Atoi(limitDays)
		if err == nil {
			if limit > 0 && limit <= len(cd) {
				cd = cd[len(cd)-limit:]
			}
		} else {
			fLog.Printf("HandleChart(), strconv.Atoi(limitDays), &cd): %v\n", err)
		}
	}

	cdBytes, err := json.Marshal(cd)
	if err != nil {
		fLog.Printf("HandleChart(), json.Marshal(cd): %v\n", err)
	}
	cdString := string(cdBytes)

	err = td.tpl.Execute(w, struct {
		Host       string
		Data       template.JS
		FontColor  string
		FontSize   string
		FontFamily string
	}{
		td.conf.ListenAddress,
		template.JS(cdString),
		td.conf.FontColor,
		td.conf.ChartFontSize,
		td.conf.ChartFontFamily,
	})
	if err != nil {
		fLog.Printf("HandleChart(), Execute(): %v\n", err)
	}
}

func (td *tplData) HandleIndex(w http.ResponseWriter, r *http.Request) {
	updateStatsFromFiles()

	var data []WeaponData
	var err error

	dataType := r.URL.Query().Get("dataType")

	if dataType == "types" {
		data = stats.Types
	} else {
		data = stats.Categories
	}

	sort.SliceStable(data, func(i, j int) bool {
		return data[i].Quantity > data[j].Quantity
	})

	limitParam := r.URL.Query().Get("limit")
	limit := 0
	if limitParam != "" {
		limit, err = strconv.Atoi(limitParam)
		if err != nil {
			fLog.Printf("HandleIndex(), strconv.Atoi(limitParam): %v\n", err)
		}
	}

	if limit > 0 && limit < len(data) {
		data = data[:limit]
	}

	err = td.tpl.Execute(w, struct {
		Host      string
		Data      []WeaponData
		FontColor string
	}{td.conf.ListenAddress, data, td.conf.FontColor})
	if err != nil {
		fLog.Printf("HandleIndex(), Execute(): %v\n", err)
	}
}
