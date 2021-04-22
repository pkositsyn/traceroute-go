package trace

import (
	"net"
	"time"
)

type Config struct {
	DstAddr   *net.UDPAddr
	DstPort   int
	NumProbes int
	Timeout   time.Duration
	MaxTTL    int
	SrcIP     *net.IPAddr

	Parallelism int
}
