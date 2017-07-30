package ds4

import (
	"github.com/jochenvg/go-udev"
	"github.com/usedbytes/input2"
)

const driverName = "DualShock 4"
type Driver struct { }

func (d Driver) Name() string {
	return driverName
}

func (d Driver) MatchDevice(a *udev.Device) bool {
	return MatchDevice(a, nil)
}

func (d Driver) CompareDevices(b *udev.Device, a *udev.Device) bool {
	return MatchDevice(a, b)
}

func (d Driver) Bind(syspath string) input2.Source {
	return NewGamepad(syspath)
}
