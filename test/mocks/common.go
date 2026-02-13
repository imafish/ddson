package mocks

import (
	"net"
	"strconv"
)

// listenOnPort creates a TCP listener on the specified port (0 for random port)
func listenOnPort(port int) (net.Listener, error) {
	addr := "127.0.0.1:" + strconv.Itoa(port)
	return net.Listen("tcp", addr)
}
