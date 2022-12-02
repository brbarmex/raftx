package server

import (
	"bufio"
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

type Server struct {
	Node           *node.Node
	portBind       string
	peers          *[]string
	failureDelay   time.Duration
	maxDelayToWait time.Duration
	listener       net.Listener
}

func NewServer() *Server {

	srv := &Server{}
	srv.portBind = "40000"
	srv.maxDelayToWait = 1 * time.Second
	srv.Node = node.NewNode()
	return srv
}

func (s *Server) Start() {

	var err error
	s.listener, err = net.Listen("tcp", "127.0.0.1:"+s.portBind)
	if err != nil {
		log.Fatalln("failure when start server node TCP; eror:", err)
	}

	defer s.listener.Close()
	log.Printf("listining on: %s \n", s.listener.Addr().String())

	quitCh := make(chan os.Signal, 1)
	signal.Notify(quitCh, os.Interrupt, os.Kill, syscall.SIGTERM)
	s.Node.SetNodeState(node.Follower)
	go s.Node.ElectionTime()

	for {

		select {
		case <-quitCh:
			s.close()
		default:
		}

		conn, err := s.accept()
		if err != nil {
			log.Fatalln("failure when accept listener connection; error: ", err.Error())
		}

		if conn == nil {
			continue
		}

		go s.hanlder(conn)
	}
}

func (s *Server) hanlder(c net.Conn) {

	defer c.Close()

	fmt.Printf("Receive message %s\n", c.RemoteAddr().String())
	for {
		netData, err := bufio.NewReader(c).ReadString('\n')
		if err != nil {
			fmt.Println(err)
			return
		}

		temp := strings.TrimSpace(string(netData))
		fmt.Println(temp)
		c.Write([]byte(`hi`))

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

		log.Printf("http: Accept error: %s; retryng in %v \n", err.Error(), s.failureDelay)
		time.Sleep(s.failureDelay)
		return nil, err
	}

	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (s *Server) close() {
	s.listener.Close()
	close(s.Node.ResetElectionTimeout)
	s.Node.ElectionTimeoutTicker.Stop()
	os.Exit(3)
}
