package selfupdate

import "time"

const (
	updateLockTimeout = 5 * time.Second
	updateLockPoll    = 25 * time.Millisecond
)
