package ds4

import (
	"fmt"
	"path/filepath"
	"github.com/usedbytes/battery"
)

func (g *Gamepad) initBattery() error {
	psus, err := filepath.Glob(g.sysdir + "/power_supply/*")
	if err != nil {
		return err
	}
	if len(psus) != 1 {
		return fmt.Errorf("Wrong number of power_supply-s")
	}
	g.battery, err = battery.NewBattery(psus[0])
	if err != nil {
		return err
	}

	return nil
}

func (g *Gamepad) GetBattery() battery.Battery {
	return g.battery
}

func (g *Gamepad) Charge() float32 {
	return g.battery.Charge()
}

func (g *Gamepad) Status() battery.Status {
	return g.battery.Status()
}
