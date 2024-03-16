package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"

	"time"

	"github.com/martinbreu/burnmaid/sensoric"
)

const (
	ALERT_URL = "http://growmpage:8080/SIM" //TODO: to setings file
	PORT      = "8088" //TODO: to setings file

	hoursToCooldown       = 10
	timeFormat            = "Mon 2.1, 15:04"
	updateRateSeconds     = 3 //TODO: also export all defaults to html files
	FILE_config_brandList = "../data/brandList.json"
	FILE_log              = "../data/burnmaid.log"
	FILE_plot             = "./script/plot.sh"
)

type Brand struct {
	Name   string
	Start  JSONTime
	Phases []Phase

	status status
}

type status struct { //TODO: divide
	measured     int
	error        error //TODO: do not store here!
	want         int
	endtime      JSONTime
	phaseInfo    string `default:"unset"`
	heating      bool
	running      bool
	errorCounter int

	indexCache *bytes.Buffer
}

type Phase struct {
	TemperatureToStart int
	TemperatureToReach int
	MinutesToHold      int
	CelciusPerHour     int

	start time.Time
	end   time.Time
}

type JSONTime struct{ time.Time }
type JSONDuration struct{ time.Duration }

func main() {
	brand := NewBurnmaid()
	brand.serve()
}

func NewBurnmaid() *Brand {
	brand := &readBrandListFromFile()[0] //top=first=default TODO: new package/file
	observer := &sensoric.Observer{}
	switcher := &sensoric.Switcher{}
	errorCounterAlert := 0

	burnTicker := time.NewTicker(updateRateSeconds * time.Second)
	go func() {
		for {
			select {
			case <-burnTicker.C:
				brand.updateStatus(observer.MeasureTemperatur())
				brand.regulate(switcher)
				if brand.status.errorCounter > errorCounterAlert {
					http.Get(ALERT_URL)
					errorCounterAlert = brand.status.errorCounter + (80 / updateRateSeconds) //TODO: magic numbers everywhere
				}
				brand.updateIndexCache()
			}
		}
	}()

	logTicker := time.NewTicker(30 * time.Second)
	go func() {
		for {
			select {
			case <-logTicker.C:
				if time.Now().Before(brand.status.endtime.Time.Add((hoursToCooldown * 2) * time.Hour)) {
					brand.logForPlot()
				}
			}
		}
	}()

	return brand
}

var measureErrorCounter int = 0

func (b *Brand) updateStatus(measured int, measureError error) {
	if measureError != nil {
		b.status.errorCounter++ //TODO: list of errors, function.
		b.status.error = measureError
		fmt.Println(b.status.error.Error())
		if measureErrorCounter <= 5 { //else error value 9999 from observer
			measured = b.status.measured
		}
	} else {
		measureErrorCounter = 0
	}

	b.status.measured = measured //9999 or real
	b.status.want = -9999

	phase := b.updatePhases()
	if phase != nil {
		b.status.running = true
		b.status.want = phase.shouldTemperature()
		if math.Abs(float64(b.status.want-b.status.measured)) > 50 {
			b.status.errorCounter++
			b.status.error = fmt.Errorf("temperatur difference above 50: %v, Zeit zurück:%v", b.status.want-b.status.measured, (b.status.want-b.status.measured) < 0)
			fmt.Println(b.status.error.Error())
			b.status.want = -9999
		}
		// if b.status.want == phase.TemperatureToReach && (b.status.want - b.status.measured) > 10{
		// 	holdedLongEnough = false
		// }else {
		// 	holdedLongEnough = true
		// }
	} else {
		if time.Now().After(b.status.endtime.Time) {
			b.status.running = false
			b.status.phaseInfo = "cooldown"
		}
		if time.Now().Before(b.Start.Time) {
			b.status.running = true
			b.status.phaseInfo = fmt.Sprintf("starting in %v minutes", int(-time.Since(b.Start.Time).Minutes()))
		}
	}
}

// var holdedLongEnough bool
func (brand *Brand) regulate(switcher *sensoric.Switcher) {
	var err error
	if brand.status.measured < brand.status.want { // || !holdedLongEnough
		err = switcher.SwitchOn()
	} else {
		err = switcher.SwitchOff()
	}
	if err != nil {
		brand.status.errorCounter++
		brand.status.error = err
		fmt.Println(brand.status.error.Error())
	}
	brand.status.heating = switcher.IsOn
}

func (b *Brand) updatePhases() *Phase {
	t := b.Start.Time
	for i, phase := range b.Phases {
		b.Phases[i].start = t
		if phase.CelciusPerHour == 0 {
			continue
		}
		total := (time.Duration(phase.MinutesToHold) * time.Minute) + (time.Duration(int(math.Abs(float64(phase.TemperatureToReach-phase.TemperatureToStart))/float64(phase.CelciusPerHour))) * time.Hour)
		t = t.Add(total)
		b.Phases[i].end = t
	}
	b.status.endtime.Time = t

	for i, phase := range b.Phases {
		if time.Now().After(phase.start) && time.Now().Before(phase.end) {
			b.status.phaseInfo = strconv.Itoa(i + 1)
			return &phase
		}
	}
	return nil
}

func (p Phase) shouldTemperature() int {
	holdStart := p.end.Add(-time.Duration(p.MinutesToHold) * time.Minute)
	isHold := time.Now().After(holdStart)
	if isHold {
		return p.TemperatureToReach
	}
	factor := time.Since(p.start).Minutes() / holdStart.Sub(p.start).Minutes()
	return p.TemperatureToStart + int(factor*float64(p.TemperatureToReach-p.TemperatureToStart))
}

func (b *Brand) logForPlot() {
	f, err := os.OpenFile(FILE_log,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		b.status.error = err
		return
	}
	defer f.Close()
	logger := log.New(f, "", log.LstdFlags)
	if time.Now().After(b.status.endtime.Time) {
		logger.Print("\t0\t" + strconv.Itoa(b.status.measured) + "\n")
	} else {
		logger.Print("\t" + strconv.Itoa(b.status.want) + "\t" + strconv.Itoa(b.status.measured) + "\n")
	}

}

func (b Brand) isInvalid() bool {
	for _, phase := range b.Phases {
		if !sensoric.InRange(phase.TemperatureToReach) {
			return true
		}
		if !sensoric.InRange(phase.TemperatureToStart) {
			return true
		}
		if phase.MinutesToHold < 0 || phase.MinutesToHold > 2000 {
			return true
		}
		if phase.CelciusPerHour < 0 || phase.CelciusPerHour > 500 {
			return true
		}
	}
	return false
}

func readBrandListFromFile() []Brand {
	file, err1 := os.ReadFile(FILE_config_brandList)
	brandList := []Brand{}
	err2 := json.Unmarshal(file, &brandList)
	if err1 != nil || err2 != nil || len(brandList) == 0 {
		fmt.Printf("\n error reading brandList.json, load default.")
		return []Brand{
			{
				Name:  "Schrüh",
				Start: JSONTime{time.Now()},
				Phases: []Phase{
					{TemperatureToStart: 20, TemperatureToReach: 960, MinutesToHold: 30, CelciusPerHour: 80},
				},
			},
		}
	} else {
		return brandList
	}
}

func (t JSONTime) String() string {
	return t.Time.Format(timeFormat)
}
func (t JSONTime) MarshalJSON() ([]byte, error) {
	stamp := fmt.Sprintf("\"%s\"", t.Time.Format(time.UnixDate))
	return []byte(stamp), nil
}
func (t *JSONTime) UnmarshalJSON(b []byte) error {
	location, _ := time.LoadLocation("Europe/Budapest")
	value := strings.Trim(string(b), `"`)
	parsed, err := time.ParseInLocation(time.UnixDate, value, location)
	if err != nil {
		fmt.Printf("ERROR UnmarshalJSON JSONTime: %v\n", err)
		t.Time = time.Now()
	} else {
		t.Time = parsed
	}
	return nil
}

func (t JSONDuration) MarshalJSON() ([]byte, error) {
	stamp := fmt.Sprintf("\"%s\"", t.String())
	return []byte(stamp), nil
}
func (d *JSONDuration) UnmarshalJSON(b []byte) error {
	value := strings.Trim(string(b), `"`)
	if value == "" || value == "null" {
		d.Duration = time.Duration(0)
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return err
	}
	d.Duration = parsed
	return nil
}
