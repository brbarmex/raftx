package server

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/brbarmex/raftx/internal/node"
	"github.com/spf13/viper"
)

var ErrTimeout = errors.New("listener timeout, retryng")

type Server struct {
	node      *node.Node
	failDelay time.Duration
	listener  net.Listener
	address   string
}

func NewServer(cfg *viper.Viper) *Server {
	return &Server{
		node:      node.NewNode(cfg),
		address:   cfg.GetString("app.address"),
		failDelay: 1 * time.Second,
		listener:  nil,
	}
}

func (s *Server) Start() {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func(ctx context.Context) {

		log.Printf("port load from env: %s \n",s.address)

		l, err := net.Listen("tcp", s.address)
		if err != nil {
			log.Fatalln(err)
		}

		defer s.close()

		go s.node.ElectionTime()

		s.listener = l
		log.Printf("listener addr: %s\n", s.listener.Addr().String())

		for {

			select {
			case <-ctx.Done():

				s.close()
				return

			default:

				conn, err := s.accept()
				if err != nil {
					if errors.Is(err, ErrTimeout) {
						ctx.Done()
						continue
					}
				}

				s.node.HandlerRequest(conn)
			}
		}

	}(ctx)

	exit := make(chan os.Signal, 1)
	signal.Notify(exit, syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGTERM)

	<-exit
	cancel()

}

func (s *Server) accept() (net.Conn, error) {

	conn, err := s.listener.Accept()
	if err == nil {
		return conn, nil
	}

	netErr, ok := err.(net.Error)
	if ok && netErr.Timeout() {
		log.Printf("Accept error :: %s; retryng in %v \n", err.Error(), s.failDelay)
		time.Sleep(s.failDelay)
		return nil, ErrTimeout
	}

	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (s *Server) close() {
	log.Println("close server ...")
	s.listener.Close()
	s.node.Close()
}
