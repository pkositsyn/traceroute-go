package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"time"

	"github.com/pkositsyn/traceroute-go/output"
	"github.com/pkositsyn/traceroute-go/trace"
)

var (
	dstPort    = flag.Int("p", 33434, "destination UDP port")
	numProbes  = flag.Int("q", 3, "number of probes for one hop")
	timeout    = flag.Duration("w", 5*time.Second, "packets timeout")
	parallelism = flag.Int("n", 16, "number of simultaneous queries")
	maxTTL     = flag.Int("m", 30, "max TTL")
	srcIP      = flag.String("s", "0.0.0.0", "source IP")
	returnJson = flag.Bool("j", false, "return result in JSON format")
)

func main() {
	flag.Parse()

	if err := validateArgs(); err != nil {
		fmt.Printf("invalid command-line arguments provided: %s\n", err.Error())
		return
	}

	dstAddr, err := extractDestinationIP()
	if err != nil {
		fmt.Printf("couldn't extract dst IP: %s\n", err.Error())
		return
	}

	srcIP, err := net.ResolveIPAddr("ip4", *srcIP)
	if err != nil {
		fmt.Printf("couldn't extract src IP: %s\n", err.Error())
		return
	}

	cfg := trace.Config{
		DstAddr:   dstAddr,
		DstPort:   *dstPort,
		NumProbes: *numProbes,
		Timeout:   *timeout,
		MaxTTL:    *maxTTL,
		SrcIP:     srcIP,

		Parallelism: *parallelism,
	}

	assembler := trace.NewAssembler(cfg)
	resultCh := assembler.Assemble()

	var formatter output.Formatter
	if *returnJson {
		formatter = new(output.JsonFormatter)
	} else {
		formatter = new(output.ConsoleFormatter)
	}

	if err := formatter.Format(resultCh); err != nil {
		fmt.Printf("error executing traceroute: %s\n", err.Error())
	}
}

func validateArgs() error {
	if *dstPort <= 0 {
		return errors.New("destination port must be a positive integer")
	}

	if *parallelism <= 0 {
		return errors.New("parallelism must be a positive integer")
	}

	if *numProbes <= 0 {
		return errors.New("number of probes must be a positive integer")
	}

	if *numProbes > 10 {
		return errors.New("number of probes cannot be more than 10")
	}

	if *maxTTL <= 0 {
		return errors.New("max hops must be a positive integer")
	}

	if *maxTTL > 255 {
		return errors.New("max hops cannot be more than 255")
	}

	if *timeout == 0 {
		return errors.New("timeout must be positive")
	}
	return nil
}

func extractDestinationIP() (*net.UDPAddr, error) {
	if flag.NArg() == 0 {
		return nil, errors.New("destination IP must be specified as a positional argument")
	}

	dstIPString := flag.Arg(0)
	dstAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", dstIPString, *dstPort))
	if err != nil {
		return nil, fmt.Errorf("cannot resolve address: %w", err)
	}
	return dstAddr, nil
}
