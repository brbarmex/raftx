package nettcp

import (
	"context"
	"net"
)

type NetTCPHandler interface {
	HandlerRequest(c net.Conn)
}

type NetTCP interface {
	Listener(ctx context.Context)
}

type netTCP struct {
	netTCPHandler NetTCPHandler
	address       string
}

func (s *netTCP) Listener(ctx context.Context) {
	listener(ctx, s.address, s.netTCPHandler.HandlerRequest)
}

func NewTCP(netTCPHandler NetTCPHandler, address string) NetTCP {
	return &netTCP{
		netTCPHandler: netTCPHandler,
		address:       address,
	}
}
