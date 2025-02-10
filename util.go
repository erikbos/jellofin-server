package main

import (
	"time"
)

func TimeToUnixMS(t time.Time) int64 {
	return t.UnixNano() / 1000000
}

func UnixMSToTime(ms int64) time.Time {
	return time.Unix(0, ms*1000000)
}
