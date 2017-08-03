package input2

import (
	"github.com/jochenvg/go-udev"
)

type InputEvent interface{}

type Connection interface{
	SetFilter(EventMatch, EventFilter)
	Subscribe(<-chan bool) <-chan InputEvent
}

type Source interface{
	NewConnection() Connection
}

type Driver interface {
	Name() string
	MatchDevice(d *udev.Device) bool
	CompareDevices(a *udev.Device, b *udev.Device) bool
	Bind(syspath string) Source
}
