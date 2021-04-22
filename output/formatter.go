package output

import "github.com/pkositsyn/traceroute-go/trace"

type Formatter interface {
	Format(channel *trace.ResponseChannel) error
}
