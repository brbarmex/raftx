package server

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/brbarmex/raftx/node"
)

var ErrorNetConnectionContinueWithFailure = errors.New("listener timeout, retryng")

type Server struct {
	Node           *node.Node
	portBind       string
	peers          []string
	failureDelay   time.Duration
	maxDelayToWait time.Duration
	listener       net.Listener
}

func NewServer() *Server {

	srv := &Server{}
	srv.portBind = "4000" . // <<- get here from env
	srv.maxDelayToWait = 1 * time.Second
	srv.Node = node.NewNode()
	srv.peers = []string{}
	srv.peers = append(srv.peers, "4321") // <<- get here from env
	return srv
}

func (s *Server) Start() {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func(ctx context.Context) {

		l, err := net.Listen("tcp", "127.0.0.1:"+s.portBind)
		if err != nil {
			log.Fatalln("failure when start server node TCP; error :: ", err)
		}

		s.listener = l

		defer s.listener.Close()
		log.Printf("listining on :: %s \n", s.listener.Addr().String())

		s.Node.SetNodeState(node.Follower)
		go s.Node.ElectionTime()

		for {

			select {
			case <-ctx.Done():

				log.Println("close")
				s.close()
				return

			default:

				conn, err := s.accept()
				if err != nil {
					if errors.Is(err, ErrorNetConnectionContinueWithFailure) {
						log.Fatalln("failure when accept listener connection; error :: ", err.Error())
						s.listener.Close()
						continue
					}
				}

				go s.hanlder(conn)
			}
		}

	}(ctx)

	exit := make(chan os.Signal, 1)
	signal.Notify(exit, syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGTERM)

	<-exit
	cancel()

	log.Println("shutdowing server")

}

func (s *Server) hanlder(c net.Conn) {

	defer c.Close()

	fmt.Printf("Receive message  ::%s\n", c.RemoteAddr().String())
	for {
		netData, err := bufio.NewReader(c).ReadString('\n')
		if err != nil {
			fmt.Println(err)
			return
		}

		temp := strings.TrimSpace(string(netData))
		fmt.Println(temp)

	}
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
	close(s.Node.ResetElectionTimeout)
	s.Node.ElectionTimeoutTicker.Stop()
}
