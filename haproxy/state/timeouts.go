package state

import "time"

var (
	connectTimeout = int64(time.Second.Seconds() * 1000)
	clientTimeout  = int64((30 * time.Second).Seconds() * 1000)
	serverTimeout  = int64((60 * time.Second).Seconds() * 1000)
)
