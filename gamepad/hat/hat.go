package hat

import (
	"time"

	"github.com/gvalkov/golang-evdev"
	"github.com/usedbytes/input2"
	"github.com/usedbytes/input2/button"
)

type Axis struct{
	Code uint16
	Invert bool
	HoldTime time.Duration

	state int32

	pos, neg button.Button
}

func (a *Axis) mapButtons(pos, neg int) {
	if a.Invert {
		a.neg = button.Button{
			HoldTime: a.HoldTime,
			Keycode: pos,
		}
		a.pos = button.Button{
			HoldTime: a.HoldTime,
			Keycode: neg,
		}
	} else {
		a.neg = button.Button{
			HoldTime: a.HoldTime,
			Keycode: neg,
		}
		a.pos = button.Button{
			HoldTime: a.HoldTime,
			Keycode: pos,
		}
	}
}

func (a *Axis) Filter(ev evdev.InputEvent, tx chan<- input2.InputEvent) {
	if ev.Value < 0 {
		a.neg.Filter(evdev.InputEvent{Code: ev.Code, Value: 1}, tx)
	} else if ev.Value > 0 {
		a.pos.Filter(evdev.InputEvent{Code: ev.Code, Value: 1}, tx)
	} else if a.state < 0 {
		a.neg.Filter(evdev.InputEvent{Code: ev.Code, Value: 0}, tx)
	} else if a.state > 0 {
		a.pos.Filter(evdev.InputEvent{Code: ev.Code, Value: 0}, tx)
	}
	a.state = ev.Value
}

type Hat struct {
	X, Y Axis
}

func (h *Hat) Filter(ev evdev.InputEvent, tx chan<- input2.InputEvent) {
	switch ev.Code {
	case h.X.Code:
		h.X.Filter(ev, tx)
	case h.Y.Code:
		h.Y.Filter(ev, tx)
	}
}

func MapHat(c input2.Connection, h *Hat) {
	(&h.X).mapButtons(evdev.KEY_RIGHT, evdev.KEY_LEFT)
	(&h.Y).mapButtons(evdev.KEY_UP, evdev.KEY_DOWN)

	c.SetFilter(input2.EventMatch{evdev.EV_ABS, h.X.Code}, h)
	c.SetFilter(input2.EventMatch{evdev.EV_ABS, h.Y.Code}, h)
}
