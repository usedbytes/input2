package input2

import (
	"github.com/jochenvg/go-udev"
	"github.com/gvalkov/golang-evdev"
)

const EvcodeAny = uint16(0xffff)

type InputEvent interface{}

type EventMatch struct {
	Type uint16
	Code uint16
}
type EventFilter interface{
	Filter(evdev.InputEvent, chan<- InputEvent)
}

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

var MatchAll = EventMatch{
	Type: evdev.EV_MAX + 1,
	Code: EvcodeAny,
}

type passthroughFilter struct {}
var PassthroughFilter passthroughFilter
func (f passthroughFilter) Filter(ev evdev.InputEvent, tx chan<- InputEvent) {
	tx<-ev
}

type dropFilter struct {}
var DropFilter dropFilter
func (f dropFilter) Filter(ev evdev.InputEvent, tx chan<- InputEvent) {
	return
}
