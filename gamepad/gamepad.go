package gamepad

import (
	"time"
	"github.com/usedbytes/linux-led"
	"github.com/usedbytes/battery"
)

type Gamepad interface {
	GetLED() led.LinuxLED
	GetBattery() battery.Battery
}

type Rumbler interface {
	CreateRumbleEffect(strongMag, weakMag float32, duration time.Duration) (RumbleEffect, error)
}

type RumbleEffect interface {
	Play() error
	Stop() error
	Delete() error
}
