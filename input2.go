package input2

import (
	"github.com/jochenvg/go-udev"
	"github.com/gvalkov/golang-evdev"
)

type Source interface {
	Subscribe(stop <-chan bool) <-chan evdev.InputEvent
}

type Driver interface {
	Name() string
	MatchDevice(d *udev.Device) bool
	CompareDevices(a *udev.Device, b *udev.Device) bool
	Bind(syspath string) Source
}
