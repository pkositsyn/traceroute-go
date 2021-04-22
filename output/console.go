package output

import (
	"fmt"
	"github.com/pkositsyn/traceroute-go/trace"
	"io"
	"net"
	"strconv"
	"strings"
)

type ConsoleFormatter struct {}

var _ Formatter = (*ConsoleFormatter)(nil)

func (c *ConsoleFormatter) Format(channel *trace.ResponseChannel) error {
	for {
		row, err := channel.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			return err
		}

		var builder strings.Builder
		builder.WriteByte(' ')
		builder.WriteString(strconv.Itoa(row.TTL))
		builder.WriteString("  ")

		for ip, value := range row.PacketsForIP {
			if ip != "None" {
				hosts, _ := net.LookupAddr(ip)
				host := ip
				if len(hosts) > 0 {
					host = hosts[0]
					if strings.HasSuffix(host, ".") {
						host = host[:len(host)-1]
					}
				}
				ip = fmt.Sprintf("%s (%s) ", host, ip)
				builder.WriteString(ip)
			}
			builder.WriteString(fmt.Sprintf("%s  ", strings.Join(value, " ")))
		}
		builder.WriteByte('\n')
		fmt.Print(builder.String())
	}
	return nil
}
