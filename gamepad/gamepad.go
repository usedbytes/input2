package gamepad

import (
	"time"
)

type Rumbler interface {
	CreateRumbleEffect(strongMag, weakMag float32, duration time.Duration) (RumbleEffect, error)
}

type RumbleEffect interface {
	Play() error
	Stop() error
	Delete() error
}
