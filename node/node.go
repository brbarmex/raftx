package node

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type NodeState int

const (
	Follower NodeState = iota + 1
	Candidate
	Leader
)

type Node struct {

	//Node Id
	Id string

	//The election timeout is randomized to be between 150ms and 300ms.
	ElectionTimeoutTicker *time.Ticker

	HeartbeatTimeoutTicker *time.Ticker

	// Reset election timeout ticker
	ResetElectionTimeout chan struct{}

	eletionTimeoutInterval int

	//The heartbeat interval, this should be less than election timeout
	heartbeatTimeoutInterval int

	//The Heartbeat timeout
	IdleTimeout int

	//The state node
	currentState NodeState

	//latest term server has seen (initialized to 0 on first boot, increases monotonically)
	currentTerm int

	Peers [1]string

	LeaderNodeId string

	rw sync.RWMutex
}

func (node *Node) SetEletionTimeout() {

	var min, max int = 149, 302
	rand.Seed(time.Now().UnixNano())
	node.eletionTimeoutInterval = rand.Intn(max-min) + min
	node.ElectionTimeoutTicker = time.NewTicker(time.Duration(node.eletionTimeoutInterval) * time.Millisecond)
}

func (node *Node) SetHeartbeatTimeout() {
	node.heartbeatTimeoutInterval = 100
}

func (node *Node) SetNodeState(state NodeState) {

	defer node.rw.Unlock()
	node.rw.Lock()
	node.currentState = state
}

func (node *Node) ElectionTime() {
	for {
		select {
		case <-node.ElectionTimeoutTicker.C:

			if node.currentState == Follower {
				node.SetNodeState(Candidate)
				go node.becomeLeader()
			}

			//fmt.Println(node.currentState)

		case <-node.ResetElectionTimeout:
			node.resetElectionTimeoutTicker()
		}
	}
}

func (node *Node) resetElectionTimeoutTicker() {
	defer node.rw.Unlock()
	node.rw.Lock()
	node.ElectionTimeoutTicker.Reset(time.Duration(node.eletionTimeoutInterval) * time.Millisecond)
}

func (node *Node) becomeLeader() {

	node.currentTerm += 1
	node.resetElectionTimeoutTicker()
	voteMessageReq := node.buildVoteRequest(node.currentTerm, node.Id, 0, 0)
	var totalVotesReceivedInFavor int

	for i := range node.Peers {

		go func(msgReq string, peer string) {

			c, err := net.Dial("tcp", peer)
			if err != nil {
				log.Println(err)
				return
			}

			// >> sending message to peer
			c.Write([]byte(msgReq + "\n"))

			go func(c net.Conn) {

				defer c.Close()

				data, err := bufio.NewReader(c).ReadString('\n')
				if err != nil {

					// need implement traitment
					return
				}

				msgResp := strings.TrimSpace(string(data))
				log.Printf("received message from peer %s :: message %s \n", peer, string(msgResp))

				splits := strings.Split(msgResp, "|")
				if len(splits) < 3 || len(splits[0]) == 0 || len(splits[1]) == 0 || len(splits[2]) == 0 {
					return
				}

				var term int
				var voteResult bool

				if term, err = strconv.Atoi(splits[2]); err != nil {
					return
				}

				if voteResult, err = strconv.ParseBool(splits[3]); err != nil {
					return
				}

				if node.currentState == Candidate && node.currentTerm == term && voteResult {
					totalVotesReceivedInFavor += 1
				}

			}(c)

		}(voteMessageReq, node.Peers[i])
	}

	if node.currentState == Leader {
		return
	}

	switch totalVotesReceivedInFavor >= (len(node.Peers)+1)/2 {
	case true:
		node.ElectionTimeoutTicker.Stop()
		node.SetNodeState(Leader)
		node.LeaderNodeId = node.Id
		go node.heartbeatTime()
	case false:
		node.SetNodeState(Follower)
	default:
		node.SetNodeState(Follower)
	}
}

func (node *Node) HandlerRequest(c net.Conn) {

	defer c.Close()

	netData, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			return
		}

		log.Println("nao consigo ler", err)
		return
	}

	temp := strings.TrimSpace(string(netData))
	log.Println("NODE B :: Received ::", temp)
	//c.Write([]byte("NODE B :: Received ::" + temp + "\n"))

}

func (node *Node) heartbeatTime() {

	node.HeartbeatTimeoutTicker = time.NewTicker(time.Duration(node.heartbeatTimeoutInterval) * time.Millisecond)
	for ticker := range node.HeartbeatTimeoutTicker.C {

		//fmt.Println("sending heartbeat at: ", ticker)

		for i := range node.Peers {

			c, err := net.Dial("tcp", node.Peers[i])
			if err != nil {
				log.Panicln("ERRO NO HEARTBEAT ", ticker.String())
				return
			}

			c.Write([]byte("RECEBA CARALHO" + "\n"))
		}

	}

}

func (node *Node) Close() {

	close(node.ResetElectionTimeout)
	node.ElectionTimeoutTicker.Stop()
	node.ElectionTimeoutTicker.Stop()
	node.HeartbeatTimeoutTicker.Stop()
}

func (node *Node) buildVoteRequest(term int, candidateId string, lastLogIndex, lastLogTerm int) string {
	return fmt.Sprintf("vote|%d|%s|%d|%d", term, candidateId, lastLogIndex, lastLogTerm)
}

func NewNode() *Node {
	node := &Node{}
	node.Id = "1"
	node.SetNodeState(Follower)
	node.SetEletionTimeout()
	node.SetHeartbeatTimeout()
	return node
}
