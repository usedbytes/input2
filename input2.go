package input2

import (
	"github.com/jochenvg/go-udev"
	"github.com/gvalkov/golang-evdev"
)

type RawEvent evdev.InputEvent
type InputEvent interface{}

type Source interface {
	Subscribe(stop <-chan bool) <-chan InputEvent
}

type Driver interface {
	Name() string
	MatchDevice(d *udev.Device) bool
	CompareDevices(a *udev.Device, b *udev.Device) bool
	Bind(syspath string) Source
}
