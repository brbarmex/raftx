package nettcp

import (
	"net"
)

func accept(l net.Listener) (net.Conn, error) {
	return l.Accept()
}
