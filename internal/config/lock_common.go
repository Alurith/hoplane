package config

import "time"

const (
	configLockTimeout = 5 * time.Second
	configLockPoll    = 25 * time.Millisecond
)
