package input2

import (
	"github.com/jochenvg/go-udev"
	"github.com/gvalkov/golang-evdev"
)

type RawEvent evdev.InputEvent
type InputEvent interface{}

type EventMatch struct {
	TypeMask uint32
	CodeMap map[uint32][]uint32
}
type FilterFunc func(ev *evdev.InputEvent) (InputEvent, bool)
type EventFilter struct {
	Match EventMatch
	Filter FilterFunc
}

type Source interface {
	Subscribe(stop <-chan bool) <-chan InputEvent
	AddFilter(filter *EventFilter)
}

type Driver interface {
	Name() string
	MatchDevice(d *udev.Device) bool
	CompareDevices(a *udev.Device, b *udev.Device) bool
	Bind(syspath string) Source
}

var PassthroughFilter = EventFilter{
	Match: EventMatch{
		TypeMask: 0xffffffff,
		CodeMap:  nil,
	},
	Filter: func(ev *evdev.InputEvent) (InputEvent, bool) {
		return *ev, true
	},
}

var DropAllFilter = EventFilter{
	Match: EventMatch{
		TypeMask: 0xffffffff,
		CodeMap:  nil,
	},
	Filter: func(ev *evdev.InputEvent) (InputEvent, bool) {
		return nil, true
	},
}
