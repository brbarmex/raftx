package node

import (
	"bufio"
	"fmt"
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

	peers []string

	isLeader bool

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

			//fmt.Println("I'm a candidate now")

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
	voteRequest := node.buildVoteRequest(node.currentTerm, node.Id, 0, 0)
	var totalVotesReceived int

	var wg sync.WaitGroup

	for i := range node.peers {

		wg.Add(1)

		go func(msgReq string, peer string) {

			defer wg.Done()

			c, err := net.Dial("tcp", peer)
			if err != nil {
				return
			}

			c.Write([]byte(msgReq + "\n"))

			go func(c net.Conn) {

				defer c.Close()
				for {

					data, err := bufio.NewReader(c).ReadString('\n')
					if err != nil {
						continue
					}

					msgResp := strings.TrimSpace(string(data))
					fmt.Println("Received message :: ", string(msgResp))

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
						totalVotesReceived += 1
					}
				}

			}(c)

		}(voteRequest, node.peers[i])
	}

	wg.Wait()

	if node.currentState == Leader {
		return
	}

	if totalVotesReceived >= (len(node.peers)+1)/2 {
		node.SetNodeState(Leader)
		node.ElectionTimeoutTicker.Stop()
		node.isLeader = true
	}

}

func (node *Node) Close() {
	node.ElectionTimeoutTicker.Stop()
}

func (node *Node) buildVoteRequest(term int, candidateId string, lastLogIndex, lastLogTerm int) string {
	return fmt.Sprintf("vote|%d|%s|%d|%d", term, candidateId, lastLogIndex, lastLogTerm)
}

func NewNode() *Node {
	node := &Node{}
	node.Id = "1"
	node.SetEletionTimeout()
	node.SetHeartbeatTimeout()
	return node
}
