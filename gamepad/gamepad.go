package gamepad

import (
	"time"
	"github.com/usedbytes/linux-led"
)

type Gamepad interface {
	GetLED() led.LinuxLED
}

type Rumbler interface {
	CreateRumbleEffect(strongMag, weakMag float32, duration time.Duration) (RumbleEffect, error)
}

type RumbleEffect interface {
	Play() error
	Stop() error
	Delete() error
}
