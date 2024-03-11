package sensoric

import (
	"encoding/json"
	"os"
	"runtime"
	"sync"

	"github.com/smarthome-go/rpirf"
)

type Switcher struct {
	mu   sync.Mutex
	IsOn bool
}

type PlugControl struct {
	Name             string
	Code             int
	PinNumberReceive int
	PinNumberSend    int
	ProtocolIndex    int
	Repeat           int
	PulseLength      int
	Length           int
}

var (
	PLUG_on  = readPlugFromFile("../data/plug_on.json")
	PLUG_off = readPlugFromFile("../data/plug_off.json")
)

func (g *Switcher) withLockContextOnPi(fn func() error) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if runtime.GOARCH == "arm" {
		return fn()
	}
	return nil
}

func readPlugFromFile(filename string) PlugControl {
	file, err := os.ReadFile(filename)
	if err != nil {
		panic("plugfile read error")
	}
	plug := &PlugControl{}
	err = json.Unmarshal(file, &plug)
	if err != nil {
		panic("plugfile convert error")
	}
	return *plug
}

func (g *Switcher) SwitchOn() error {
	g.IsOn = true
	return g.switchPlug(PLUG_on)
}
func (g *Switcher) SwitchOff() error {
	g.IsOn = false
	return g.switchPlug(PLUG_off)
}

func (g *Switcher) switchPlug(plug PlugControl) error {
	// fmt.Println("start sending plug code: " + growhelper.ToString(plug.Code))
	return g.withLockContextOnPi(func() error {
		device, err := rpirf.NewRF(
			uint8(plug.PinNumberSend),
			uint8(plug.ProtocolIndex),
			uint8(plug.Repeat),
			uint16(plug.PulseLength),
			uint8(plug.Length),
		)
		if err != nil {
			return err
		}
		if err = device.Send(plug.Code); err != nil {
			return err
		}
		if err := device.Cleanup(); err != nil {
			return err
		}
		return nil
	})

}
