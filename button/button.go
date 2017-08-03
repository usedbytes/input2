package button

import (
	"time"
	"github.com/gvalkov/golang-evdev"
	"github.com/usedbytes/input2"
)

type Value int
type Event struct {
	Keycode int
	Value Value
}
const (
	Pressed Value = iota
	Held
)

type Button struct {
	Match    input2.EventMatch
	HoldTime time.Duration
	Keycode  int

	timer *time.Timer
}

func (b *Button) Filter(ev evdev.InputEvent, tx chan<- input2.InputEvent) {
	switch ev.Value {
		case 0:
			if b.timer != nil && !b.timer.Stop() {
				// We already sent a "held" event
				return
			}
			tx <- Event{ b.Keycode, Pressed }
		case 1:
			b.timer = time.AfterFunc(b.HoldTime, func() {
				tx <- Event{ b.Keycode, Held }
			})
	}
}

func MapButton(c input2.Connection, b *Button) {
	c.SetFilter(b.Match, b)
}
