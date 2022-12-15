package nettcp

import (
	"context"
	"log"
	"net"
	"time"
)

const (
	network      = "tcp"
	failureDelay = 1 * time.Second
)

type TCPHandler func(c net.Conn)

func listener(ctx context.Context, address string, handler TCPHandler) {

	l, err := net.Listen(network, address)
	if err != nil {
		log.Fatalf("fatal error when listener run; error: %v \n", err)
	}

	defer l.Close()

	log.Printf("start listener at %s \n", l.Addr().String())

	for {

		select {
		case <-ctx.Done():

			log.Println("close the tcp listener ...")
			return

		default:

			c, err := accept(l)
			if err != nil {

				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					log.Printf("network timeout; retrying call after delay failure og the 1sec; error :: %s \n", err.Error())
					time.Sleep(failureDelay)
					continue
				}

				ctx.Done()
				break
			}

			handler(c)
		}
	}
}
