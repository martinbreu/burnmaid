package sensoric

import (
	"errors"
	"math"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

var (
	filename_growobserver   = "./script/observer.py"
	SensorMin               = -51  //Laut Adafruit bis -199, getestet ab 0, früher war 0=faulty state, jetzt kommt fehlermeldung
	SensorMax               = 1340 //1023 mit MAX6 8bit und MAX31855 getestet bis 1310, angeblich bis 1349, Ofen Max nicht gefunden(vermutlich 1300)
	failTemperatureToReturn = 9999
)

type Observer struct {
	mu sync.Mutex
}

func (g *Observer) MeasureTemperatur() (int, error) {
	// if true {
	// 	return 0, errors.New("test")
	// }
	g.mu.Lock()
	defer g.mu.Unlock()
	if runtime.GOARCH != "arm" { //not on Pi
		return 55, nil
		// return 155, nil
	}
	command := exec.Command("python3", filename_growobserver)
	out, err := command.CombinedOutput()
	if err != nil {
		return failTemperatureToReturn, err
	}
	// fmt.Printf("string(out): %v\n", string(out))
	pythonResult := strings.TrimSpace(string(out))
	if pythonResult == "" || pythonResult == "NaN" || pythonResult == "0.0" { //TODO: "0.0" is new, why?
		return failTemperatureToReturn, errors.New("pythonResult from observer.py was empty,NaN or 0.0: "+pythonResult)
	}
	temperatureFloat, err := strconv.ParseFloat(pythonResult, 32)
	if err != nil {
		return failTemperatureToReturn, errors.New("parse error from observer: " + err.Error())
	}

	temperature := int(math.Round(temperatureFloat))
	if !InRange(temperature) {
		return failTemperatureToReturn, errors.New(pythonResult + " not between " + strconv.Itoa(SensorMin+1) + " and " + strconv.Itoa(SensorMax-1))
	}
	return temperature, nil
}

func InRange(temperature int) bool {
	return temperature > SensorMin && temperature < SensorMax
}
