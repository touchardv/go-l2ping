//
// Go implementation (conversion) of the BlueZ "l2ping" command, implemented in C
// (https://github.com/bluez/bluez/blob/master/tools/l2ping.c).
//
package main

import (
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/pflag"
	"golang.org/x/sys/unix"
)

const (
	l2CAPCommandHeaderSize     = 4
	l2CAPCommandRejectResponse = 0x01
	l2CAPEchoRequest           = 0x08
	l2CAPEchoResponse          = 0x09
)

const (
	ident byte = 200
)

var (
	bdaddr  unix.SockaddrL2
	count   int
	delay   int
	size    int
	timeout int
	sentPkt int
	recvPkt int
)

func ba2str(sa unix.Sockaddr) string {
	ba := sa.(*unix.SockaddrL2)
	var s strings.Builder
	for i := len(ba.Addr); i > 0; i-- {
		if i != len(ba.Addr) {
			s.WriteString(":")
		}
		s.WriteString(fmt.Sprintf("%02X", ba.Addr[i-1]))
	}
	return s.String()
}

func str2ba(addr string) unix.SockaddrL2 {
	a := strings.Split(addr, ":")
	var b [6]byte
	for i, tmp := range a {
		u, _ := strconv.ParseUint(tmp, 16, 8)
		b[i] = byte(u)
	}
	return unix.SockaddrL2{
		Addr: b,
		PSM:  1,
	}
}

func stat(sig int) {
	var loss int
	if sentPkt != 0 {
		loss = int(float32(sentPkt-recvPkt) / (float32(sentPkt) / 100.0))
	} else {
		loss = 0
	}
	fmt.Printf("%d sent, %d received, %d%% loss\n", sentPkt, recvPkt, loss)
	os.Exit(0)
}

func ping(svr string) {
	var localAddr unix.Sockaddr
	var addr unix.SockaddrL2
	var str string
	var id byte

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-c
		stat(0)
	}()

	sendBuff := make([]byte, l2CAPCommandHeaderSize+size, l2CAPCommandHeaderSize+size)
	receiveBuff := make([]byte, l2CAPCommandHeaderSize+size, l2CAPCommandHeaderSize+size)

	// Create socket
	sk, err := unix.Socket(unix.AF_BLUETOOTH, unix.SOCK_RAW, unix.BTPROTO_L2CAP)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Can't create socket: ", err)
		goto onError
	}
	defer unix.Close(sk)

	// Bind to local address
	err = unix.Bind(sk, &bdaddr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Can't bind socket: ", err)
		goto onError
	}

	// Connect to the remote device
	addr = str2ba(svr)
	err = unix.Connect(sk, &addr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Can't connect: ", err)
		goto onError
	}

	// Get local address
	localAddr, err = unix.Getsockname(sk)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Can't get local address: ", err)
		goto onError
	}

	str = ba2str(localAddr)
	fmt.Printf("Ping: %s from %s (data size %d) ...\n", svr, str, size)

	// Initialize send buffer
	for i := 0; i < size; i++ {
		sendBuff[l2CAPCommandHeaderSize+i] = byte(i%40 + 'A')
	}

	sendBuff[0] = l2CAPEchoRequest
	sendBuff[2] = byte(size)
	sendBuff[3] = 0

	id = ident

	for {
		if count != -1 {
			count--
			if count < 0 {
				break
			}
		}

		// Build command header
		sendBuff[1] = byte(id)

		tvSend := time.Now()

		// Send Echo Command
		_, err := unix.Write(sk, sendBuff)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Write failed: ", err)
			goto onError
		}

		// Wait for Echo Response
		lost := false
		for {
			fds := make([]unix.PollFd, 1)
			fds[0].Fd = int32(sk)
			fds[0].Events = unix.POLLIN
			n, err := unix.Poll(fds, timeout*1000)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Poll failed: ", err)
				goto onError
			}
			if n == 0 {
				lost = true
				break
			}

			n, _, err = unix.Recvfrom(sk, receiveBuff, unix.MSG_WAITALL)
			if err != nil {
				fmt.Fprintln(os.Stderr, "Recvfrom failed: ", err)
				goto onError
			}
			if n == 0 {
				fmt.Fprintln(os.Stderr, "Disconnected")
				goto onError
			}

			// Check for our id
			if receiveBuff[1] != id {
				continue
			}

			// Check type
			if receiveBuff[0] == l2CAPEchoResponse {
				break
			}

			if receiveBuff[0] == l2CAPCommandRejectResponse {
				fmt.Fprintln(os.Stderr, "Peer doesn't support Echo packets")
				goto onError
			}
		}

		sentPkt++

		if lost == false {
			recvPkt++

			tvRecv := time.Now()
			tvDiff := tvRecv.Sub(tvSend)
			fmt.Printf("%d bytes from %s id %d time %.2dms\n", receiveBuff[2], svr, id-ident, tvDiff.Milliseconds())

			if delay > 0 {
				time.Sleep(time.Duration(delay) * time.Second)
			}
		} else {
			fmt.Printf("no response from %s: id %d\n", svr, id-ident)
		}

		id++
		if id > 254 {
			id = ident
		}
	}
	stat(0)
	return

onError:
	os.Exit(1)
}

func main() {
	pflag.IntVarP(&size, "size", "s", 44, "The size of the data packets to be sent.")
	pflag.IntVarP(&count, "count", "c", -1, "Send count number of packets then exit.")
	pflag.IntVarP(&timeout, "timeout", "t", 10, "Wait timeout seconds for the response.")
	pflag.IntVarP(&delay, "delay", "d", 1, "Wait delay seconds between pings.")
	pflag.Parse()
	if len(pflag.Args()) != 1 {
		pflag.Usage()
		os.Exit(1)
	}

	bdaddr = unix.SockaddrL2{
		Addr: [6]uint8{0, 0, 0, 0, 0, 0}, // BDADDR_ANY
	}

	ping(pflag.Args()[0])
}
