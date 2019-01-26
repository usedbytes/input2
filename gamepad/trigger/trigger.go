package trigger

import (
	"fmt"

	"github.com/gvalkov/golang-evdev"
	"github.com/usedbytes/input2"
)

const (
	Left int = iota
	Right
)

type Event struct {
	Code int
	Value float32
}

func (e Event) String() string {
	return fmt.Sprintf("%d %1.2f\n", e.Code, e.Value)
}

type Trigger struct{
	Axis uint16
	Code int
}

func (t *Trigger) Filter(ev evdev.InputEvent, tx chan<- input2.InputEvent) {
	tx <-Event{
		Code: t.Code,
		Value: float32(ev.Value) / 255.0,
	}
}

func MapTrigger(c input2.Connection, t *Trigger) {
	c.SetFilter(input2.EventMatch{evdev.EV_ABS, t.Axis}, t)
}
