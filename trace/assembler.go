package trace

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"
)

type Assembler struct {
	config Config

	socketDstPortToPacket map[int]packetInfo
	packetInfoLock        sync.RWMutex
}

type packetInfo struct {
	id       int
	ttl      int
	sendTime time.Time
}

func NewAssembler(cfg Config) *Assembler {
	return &Assembler{config: cfg, socketDstPortToPacket: make(map[int]packetInfo)}
}

func (a *Assembler) Assemble() *ResponseChannel {
	responseCh := newResponseChannel()
	go a.assemble(responseCh)
	return responseCh
}

func (a *Assembler) assemble(responseCh *ResponseChannel) {
	events := make(chan event, a.config.Parallelism+1)

	go a.collectResponse(events, responseCh)

	if err := a.listenICMP(a.config.SrcIP.String(), events); err != nil {
		responseCh.errors <- err
		return
	}

	dstPort := a.config.DstPort
	semaphore := make(chan struct{}, a.config.Parallelism)
	for ttl := 1; ttl <= a.config.MaxTTL; ttl++ {
		for i := 0; i < a.config.NumProbes; i++ {
			semaphore <- struct{}{}
			if err := a.connectionCheck(ttl, dstPort, semaphore, events); err != nil {
				responseCh.errors <- err
				return
			}
			dstPort++
		}
	}
}

func (a *Assembler) collectResponse(events <-chan event, responseCh *ResponseChannel) {
	defer func() { responseCh.errors <- io.EOF }()
	response := newAssembleResponse(a.config.MaxTTL)
	processedEvents := make(map[int]struct{})
	lastTTL := a.config.MaxTTL
	var currentRow int
	for e := range events {
		responseBucket := response.Data[e.sendTTL-1].PacketsForIP
		if _, ok := processedEvents[e.id]; ok {
			continue
		}
		processedEvents[e.id] = struct{}{}

		switch e.eType {
		case eventNoResponse:
			responseBucket["None"] = append(responseBucket["None"], "*")
		case eventUnavailable:
			if e.sendTTL < lastTTL {
				lastTTL = e.sendTTL
			}
			fallthrough
		case eventTimeExceeded:
			sourceIP := e.source.String()
			sourceIP  = strings.Split(sourceIP, ":")[0]

			responseBucket[sourceIP] = append(responseBucket[sourceIP], e.rtt.String())
		}

		for currentRow < lastTTL {
			var probesForTTL int

			for _, packetsForIP := range response.Data[currentRow].PacketsForIP {
				probesForTTL += len(packetsForIP)
			}
			if probesForTTL == a.config.NumProbes {
				responseCh.ch <- response.Data[currentRow]
				currentRow++
			} else {
				break
			}
		}

		if currentRow == lastTTL {
			break
		}
	}
}

func (a *Assembler) listenICMP(address string, events chan<- event) error {
	listener, err := net.ListenPacket("ip4:icmp", address)
	if err != nil {
		return err
	}

	go a.serveICMP(listener, events)
	return nil
}

func (a *Assembler) serveICMP(listener net.PacketConn, events chan<- event) {
	defer listener.Close()
	packetBuf := make([]byte, 1024)
	for {
		n, fromAddr, err := listener.ReadFrom(packetBuf)
		if err != nil {
			fmt.Printf("error reading ICMP messages: %s\n", err.Error())
			continue
		}
		now := time.Now()

		// Need headers at least
		if n < 32 {
			continue
		}

		// Not ICMP TimeExceeded
		if packetBuf[0] != 11 {
			continue
		}

		if !bytes.Equal(packetBuf[24:28], a.config.DstAddr.IP.To4()) {
			// Wrong header format or packet for another destination
			continue
		}

		dstPort := int(binary.BigEndian.Uint16(packetBuf[30:32]))

		a.packetInfoLock.RLock()
		packetInfo := a.socketDstPortToPacket[dstPort]
		a.packetInfoLock.RUnlock()

		events <- event{
			eType:   eventTimeExceeded,
			id:      packetInfo.id,
			source:  fromAddr,
			sendTTL: packetInfo.ttl,
			rtt:     now.Sub(packetInfo.sendTime),
		}
	}
}

func (a *Assembler) connectionCheck(ttl, dstPort int, semaphore chan struct{}, events chan<- event) error {
	sendSocket, err := a.sendSocket(ttl, dstPort)
	if err != nil {
		return err
	}

	go a.connectionServe(ttl, dstPort, sendSocket, semaphore, events)
	return nil
}

func (a *Assembler) connectionServe(ttl, dstPort int, socket *net.UDPConn, semaphore chan struct{}, events chan<- event) {
	defer func() {
		<-semaphore
		socket.Close()
	}()
	a.packetInfoLock.Lock()
	sendTime := time.Now()
	a.socketDstPortToPacket[dstPort] = packetInfo{
		id:       dstPort,
		ttl:      ttl,
		sendTime: sendTime,
	}
	a.packetInfoLock.Unlock()

	socket.Write([]byte{})

	packetBuf := make([]byte, 1024)
	_, _, err := socket.ReadFrom(packetBuf)
	now := time.Now()
	ev := event{
		id:      dstPort,
		source:  a.config.DstAddr,
		sendTTL: ttl,
		rtt:     now.Sub(sendTime),
	}
	if errors.Is(err, os.ErrDeadlineExceeded) {
		ev.eType = eventNoResponse
	} else if strings.HasSuffix(err.Error(), "connection refused") {
		ev.eType = eventUnavailable
	} else {
		fmt.Printf("expected error reading from unavailable socket, got %s\n", err.Error())
		return
	}

	events <- ev
}

func (a *Assembler) sendSocket(ttl, dstPort int) (*net.UDPConn, error) {
	socket, err := net.DialUDP("udp4",
		&net.UDPAddr{IP: a.config.SrcIP.IP},
		&net.UDPAddr{
			IP:   a.config.DstAddr.IP,
			Port: dstPort,
		},
	)
	if err != nil {
		return nil, err
	}

	if err = socket.SetReadDeadline(time.Now().Add(a.config.Timeout)); err != nil {
		return nil, err
	}

	rawConn, err := socket.SyscallConn()
	if err != nil {
		return nil, err
	}

	return socket, rawConn.Control(func(fd uintptr) {
		if err = syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_TTL, ttl); err != nil {
			fmt.Println("couldn't assign TTL to socket")
		}
	})
}
