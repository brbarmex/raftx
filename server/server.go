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

	"github.com/brbarmex/raftx/node"
)

var ErrorNetConnectionContinueWithFailure = errors.New("listener timeout, retryng")

type Server struct {
	Node           *node.Node
	portBind       string
	failureDelay   time.Duration
	maxDelayToWait time.Duration
	listener       net.Listener
}

func NewServer() *Server {

	var peers []string
	peers[0] = "127.0.0.1:9876"

	srv := &Server{}
	srv.portBind = "127.0.0.1:4000" // <<- get here from env
	//srv.portBind = "127.0.0.1:9876"
	srv.maxDelayToWait = 1 * time.Second
	srv.Node = node.NewNode("1", peers)
	//srv.Node.Peers[0] = "127.0.0.1:9876" // <<- get here from env
	//srv.Node.Peers[0] = "127.0.0.1:4000"
	return srv
}

func (s *Server) Start() {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func(ctx context.Context) {

		l, err := net.Listen("tcp", s.portBind)
		if err != nil {
			log.Fatalln(err)
		}

		s.listener = l
		defer s.close()

		log.Printf("listener addr: %s\n", s.listener.Addr().String())

		go s.Node.ElectionTime()

		for {

			select {
			case <-ctx.Done():

				s.close()
				return

			default:

				conn, err := s.accept()
				if err != nil {
					if errors.Is(err, ErrorNetConnectionContinueWithFailure) {
						s.listener.Close()
						continue
					}
				}

				s.Node.HandlerRequest(conn)
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

		if s.failureDelay == 0 {
			s.failureDelay = 5 * time.Millisecond
		} else {
			s.failureDelay *= 2
		}

		if s.failureDelay > s.maxDelayToWait {
			s.failureDelay = s.maxDelayToWait
		}

		log.Printf("Accept error :: %s; retryng in %v \n", err.Error(), s.failureDelay)
		time.Sleep(s.failureDelay)
		return nil, ErrorNetConnectionContinueWithFailure
	}

	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (s *Server) close() {
	s.listener.Close()
	s.Node.Close()
}
