package thumbstick

import (
	"fmt"
	"math"

	"github.com/gvalkov/golang-evdev"
	"github.com/usedbytes/input2"
)

type Stick int
type Event struct {
	Stick Stick
	Arg float64
	Theta int16
}
const (
	Left Stick = iota
	Right
)

func (e Event)String() string {
	return fmt.Sprintf("%d %1.2f:%3d\n", e.Stick, e.Arg, e.Theta)
}

type Axis struct{
	Code uint16
	Invert bool

	offset int32
	scale int32
}

type StickAlgorithm interface{
	Sample(x, y float64) (arg float64, theta int16)
}

type Thumbstick struct {
	X, Y Axis
	Stick Stick
	Algo StickAlgorithm

	x, y float64
}

type CrossDeadzone struct{
	XDeadzone, YDeadzone float64
}

func clipCross(val, dz float64) float64 {
	switch {
	case val > -dz && val < dz:
		return 0
	case val < 0:
		return (val + dz) / (1.0 - dz)
	case val > 0:
		return (val - dz) / (1.0 - dz)
	}

	return 0
}

func (c CrossDeadzone) Sample(x, y float64) (arg float64, theta int16) {
	x = clipCross(x, c.XDeadzone)
	y = clipCross(y, c.YDeadzone)

	arg = math.Hypot(x, y)
	if arg > 1.0 {
		arg = 1.0
	}

	theta = int16(math.Atan2(x, y) * (180 / math.Pi))
	if theta < 0 {
		theta += 360
	}

	return arg, theta
}

type RadialDeadzone struct{
	Deadzone float64
}

func (r RadialDeadzone) Sample(x, y float64) (arg float64, theta int16) {
	arg = math.Hypot(x, y)
	if arg < r.Deadzone {
		return 0, 0
	}

	arg = (arg - r.Deadzone) / (1.0 - r.Deadzone)
	if arg > 1.0 {
		arg = 1.0
	}

	theta = int16(math.Atan2(x, y) * (180 / math.Pi))
	if theta < 0 {
		theta += 360
	}

	return arg, theta
}

func (a Axis) normalise(val int32) float64 {
	ret := float64((val - a.offset) * a.scale) / 32768.0
	if (a.Invert) {
		return -ret
	}
	return ret
}

func (ts *Thumbstick) Filter(ev evdev.InputEvent, tx chan<- input2.InputEvent) {
	switch ev.Code {
	case ts.X.Code:
		ts.x = ts.X.normalise(ev.Value)
	case ts.Y.Code:
		ts.y = ts.Y.normalise(ev.Value)
	}
}

func (ts *Thumbstick) Sync(ev evdev.InputEvent, tx chan<- input2.InputEvent) {
	arg, theta := ts.Algo.Sample(ts.x, ts.y)

	tx <-Event{
		Stick: ts.Stick,
		Arg: arg,
		Theta: theta,
	}
}

func MapThumbstick(c input2.Connection, ts *Thumbstick) {
	ts.X.offset = 128
	ts.X.scale = (32767 / 128)
	ts.Y.offset = 128
	ts.Y.scale = (32767 / 128)

	c.SetFilter(input2.EventMatch{evdev.EV_ABS, ts.X.Code}, ts)
	c.SetFilter(input2.EventMatch{evdev.EV_ABS, ts.Y.Code}, ts)
}
