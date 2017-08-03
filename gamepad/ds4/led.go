package ds4

import (
	"fmt"
	"image/color"
	"path/filepath"
	"strings"
	"github.com/usedbytes/linux-led"
)

func (g *Gamepad) initLeds() error {
	var red, green, blue, global string
	leds, err := filepath.Glob(g.sysdir + "/leds/*")
	if err != nil {
		return err
	}
	if len(leds) != 4 {
		return fmt.Errorf("Expected 4 LEDs")
	}

	for _, l := range leds {
		if strings.HasSuffix(l, ":red") {
			red = l
			continue
		}
		if strings.HasSuffix(l, ":green") {
			green = l
			continue
		}
		if strings.HasSuffix(l, ":blue") {
			blue = l
			continue
		}
		if strings.HasSuffix(l, ":global") {
			global = l
			continue
		}
	}
	if red == "" || green == "" || blue == "" || global == "" {
		return fmt.Errorf("Couldn't match LED names")
	}

	g.led, err = led.NewRGBLED(red, green, blue, global)
	if (err != nil) {
		return fmt.Errorf("Couldn't get leds: %s\n", err.Error())
	}

	return nil
}

func (g *Gamepad) SetColor(color color.Color) error {
	return g.led.SetColor(color)
}

func (g *Gamepad) GetColor() color.Color {
	return g.led.GetColor()
}

func (g *Gamepad) SetBrightness(brightness float32) error {
	return g.led.SetBrightness(brightness)
}

func (g *Gamepad) Off() error {
	return g.led.Off()
}

func (g *Gamepad) SetTrigger(trigger led.Trigger) error {
	return g.led.SetTrigger(trigger)
}

func (g *Gamepad) GetTrigger() led.Trigger {
	return g.led.GetTrigger()
}
