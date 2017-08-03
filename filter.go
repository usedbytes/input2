package input2

import (
	"github.com/gvalkov/golang-evdev"
)

const EvcodeAny = uint16(0xffff)

type EventMatch struct {
	Type uint16
	Code uint16
}

type EventFilter interface{
	Filter(evdev.InputEvent, chan<- InputEvent)
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
