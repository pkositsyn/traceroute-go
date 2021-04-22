package trace

import (
	"net"
	"time"
)

type eventType int

const (
	eventTimeExceeded eventType = iota
	eventUnavailable
	eventNoResponse
)

type event struct {
	eType   eventType
	id      int
	source  net.Addr
	sendTTL int
	rtt     time.Duration
}
