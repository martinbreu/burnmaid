package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"net/http"
	"os"
	"os/exec"

	"time"
)

const htmlLayout = "2006-01-02T15:04"

var indexTemplate = template.Must(template.ParseFiles("./html/index.html"))

func (b *Brand) serve() {
	b.status.indexCache = new(bytes.Buffer) //TODO: load defaults in one place
	indexTemplate.Execute(b.status.indexCache, nil)

	http.HandleFunc("/running", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, b.status.running) }) //health, checker.go at page
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		selectedConfig = 0
		fmt.Fprint(w, b.status.indexCache)
	})
	http.HandleFunc("/logo", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=99999, public")
		http.ServeFile(w, r, "./html/logo.jpg")
	})
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "./html/favicon.ico") })
	http.HandleFunc("/plot", b.plot)
	http.HandleFunc("/plot.png", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=60, public")
		os.Remove("./plot.png")
		plotLayout := "2006/01/02 15:04:05" //TODO: filewatcher ore whatever instead because loads two times if zoomed
		exec.Command(FILE_plot, plotFrom.Format(plotLayout), plotTo.Format(plotLayout)).Run()
		http.ServeFile(w, r, "./plot.png")
	})
	http.HandleFunc("/settings", b.settings)
	http.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
		lastUpdateInfo = b.update(r)
		if lastUpdateInfo == "updated" { //TODO: create restart function, also *b = *...
			b.status.indexCache = new(bytes.Buffer)
			indexTemplate.Execute(b.status.indexCache, nil)
			lastUpdateInfo = ""
			b.status.error = nil
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
	})

	hostname, _ := os.Hostname()
	fmt.Printf("open http://%v:%v\n", hostname, PORT)
	srv := &http.Server{Addr: fmt.Sprintf(":%v", PORT)}
	err := srv.ListenAndServe()
	if err != nil {
		fmt.Println(err)
	}
}

func (b *Brand) updateIndexCache() {
	var data = struct {
		Start        string
		End          string
		CoolEnd      string
		Name         string
		WantDiff     int
		Measured     int
		Info         string
		Error        string
		Ofen         bool
		ErrorCounter int
		Running      bool
	}{
		Start:        DateToGerman(b.Start.String()),
		End:          DateToGerman(b.status.endtime.String()),
		CoolEnd:      DateToGerman(b.status.endtime.Time.Add(hoursToCooldown * time.Hour).Format(timeFormat)),
		Name:         b.Name,
		WantDiff:     b.status.want - b.status.measured,
		Measured:     b.status.measured,
		Info:         b.status.phaseInfo,
		Running:      b.status.running,
		Error:        "",
		Ofen:         b.status.heating,
		ErrorCounter: b.status.errorCounter,
	}
	if b.status.error != nil {
		data.Error = time.Now().Format(timeFormat) + ": " + b.status.error.Error()
	}
	b.status.indexCache = new(bytes.Buffer)
	indexTemplate.Execute(b.status.indexCache, data)
}

// TODO:
var plotFrom time.Time
var plotTo time.Time

func (b *Brand) plot(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		fmt.Fprintf(os.Stdout, "ParseFormPlot() err: %v", err)
	}
	location, err := time.LoadLocation("Europe/Budapest")
	plotFrom, err = time.ParseInLocation(htmlLayout, r.FormValue("from"), location)
	if err != nil {
		plotFrom = b.Start.Time
	}
	plotTo, err = time.ParseInLocation(htmlLayout, r.FormValue("to"), location)
	if err != nil {
		plotTo = b.status.endtime.Time.Add(hoursToCooldown * time.Hour)
	}

	data := struct {
		From string
		To   string
	}{
		From: plotFrom.Format(htmlLayout),
		To:   plotTo.Format(htmlLayout),
	}
	fmt.Fprint(w, getPage("./html/plot.html", data))
}

func (b *Brand) settings(w http.ResponseWriter, r *http.Request) {
	list := readBrandListFromFile()
	data := struct {
		BrandList      []Brand
		Selected       int
		Name           string
		Start          string
		Phases         []Phase
		LastUpdateInfo string
		Ende           string
	}{
		BrandList:      list,
		Selected:       selectedConfig,
		Name:           list[selectedConfig].Name,
		Start:          list[selectedConfig].Start.Time.Format(htmlLayout),
		Phases:         list[selectedConfig].Phases,
		LastUpdateInfo: lastUpdateInfo,
		Ende:           b.status.endtime.Time.Add(hoursToCooldown * time.Hour).Format(timeFormat),
	}
	fmt.Fprint(w, getPage("./html/settings.html", data))
}

var selectedConfig int = 0
var lastUpdateInfo string = ""

func (b *Brand) update(r *http.Request) string {
	if r.ContentLength == 0 {
		return "nothing happend, no input"
	}
	if err := r.ParseForm(); err != nil {
		return "nothing happend, ParseForm() err: " + err.Error()
	}
	formAddPhase := r.FormValue("addPhase") == "true"
	formDeletePhase := r.FormValue("deletePhase") == "true"
	formDelete := r.FormValue("delete") == "true"
	formSelect := r.FormValue("selectTriggered") == "true"
	selectedConfig = ToInt(r.FormValue("select"))

	if formDelete {
		readBrandListFromFile()[selectedConfig].remove()
		return "removed from list"
	}
	newBrand := readBrandFromForm(r)
	if newBrand == nil {
		return "Settings unchanged because of format error"
	}

	lastTemp := newBrand.Phases[len(newBrand.Phases)-1].TemperatureToReach
	if formAddPhase {
		newBrand.Phases = append(newBrand.Phases, Phase{TemperatureToStart: lastTemp, TemperatureToReach: lastTemp + 100, CelciusPerHour: 60})
	}
	if formDeletePhase && len(newBrand.Phases) >= 1 {
		newBrand.Phases = newBrand.Phases[:len(newBrand.Phases)-1]
	}
	newBrand.saveToFile()
	if formSelect || formAddPhase || formDeletePhase {
		return ""
	}
	if newBrand.isInvalid() {
		return "Settings unchanged because configuration was invalid."
	}
	// if newBrand.Marshal() == b.Marshal() {
	// 	return "Settings unchanged"
	// }

	selectedConfig = 0
	newBrand.toTopOfFile()
	*b = *newBrand
	return "updated"
}

func readBrandFromForm(r *http.Request) *Brand {
	newBrand := &Brand{}
	location, _ := time.LoadLocation("Europe/Budapest")
	htmlLayout := "2006-01-02T15:04"
	start, err := time.ParseInLocation(htmlLayout, r.FormValue("Start"), location)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	} else {
		newBrand.Start.Time = start
	}
	newBrand.Name = r.FormValue("Name")
	//TODO: refactor, generify for page, add validation if arrays different len, ...
	for key, values := range r.Form {
		if key == "CelciusPerHour" {
			for i, value := range values {
				newBrand.Phases = append(newBrand.Phases, Phase{})
				newBrand.Phases[i].CelciusPerHour = ToInt(value)
			}
			break
		}
	}
	for key, values := range r.Form {
		if key == "TemperatureToStart" {
			for i, value := range values {
				newBrand.Phases[i].TemperatureToStart = ToInt(value)
			}
		}
		if key == "TemperatureToReach" {
			for i, value := range values {
				newBrand.Phases[i].TemperatureToReach = ToInt(value)
			}
		}
		if key == "MinutesToHold" {
			for i, value := range values {
				newBrand.Phases[i].MinutesToHold = ToInt(value)
			}
		}
	}
	return newBrand
}

func (b *Brand) toTopOfFile() {
	b.remove()
	brandList := readBrandListFromFile()
	brandList = append([]Brand{*b}, brandList...)
	err := os.WriteFile(FILE_config_brandList, marshal(brandList), 0644)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}
}

func (b *Brand) remove() {
	selectedConfig = 0
	brandList := readBrandListFromFile()
	for i, brand := range brandList {
		if brand.Name == b.Name {
			brandList = append(brandList[:i], brandList[i+1:]...)
		}
	}
	err := os.WriteFile(FILE_config_brandList, marshal(brandList), 0644)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}
}

func (b *Brand) saveToFile() {
	isUpdate := false
	brandList := readBrandListFromFile()
	for i, brand := range brandList {
		if brand.Name == b.Name {
			isUpdate = true
			brandList[i] = *b
		}
	}
	if !isUpdate {
		brandList = append(brandList, *b)
	}
	err := os.WriteFile(FILE_config_brandList, marshal(brandList), 0644)
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}
}

func ToInt(string string) int {
	value, err := strconv.Atoi(strings.TrimSpace(string))
	if err != nil {
		fmt.Printf("err: %v\n", err)
	}
	return value
}
func getPage(htmlFile string, data any) *bytes.Buffer {
	tmpl := template.Must(template.ParseFiles(htmlFile))
	out := new(bytes.Buffer)
	err := tmpl.Execute(out, data)
	if err != nil {
		fmt.Printf("fillTemplate err: %v\n", err)
	}
	return out
}

func (b *Brand) Marshal() string {
	return string(marshal(b))
}
func marshal(v any) []byte {
	bytes, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		fmt.Println(err)
	}
	return bytes
}
func DateToGerman(message string) string {
	message = strings.Replace(message, "Mon", "Montag", 1)
	message = strings.Replace(message, "Tue", "Dienstag", 1)
	message = strings.Replace(message, "Wed", "Mittwoch", 1)
	message = strings.Replace(message, "Thu", "Donnerstag", 1)
	message = strings.Replace(message, "Fri", "Freitag", 1)
	message = strings.Replace(message, "Sat", "Samstag", 1)
	message = strings.Replace(message, "Son", "Sonntag", 1)
	return message
}
