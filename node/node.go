package node

import (
	"fmt"
	"math/rand"
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

	HeartbeatTimeoutInterval int

	// Reset election timeout ticker
	ResetElectionTimeout chan struct{}

	eletionTimeoutInterval int

	//The Heartbeat timeout
	IdleTimeout int

	//The state node
	currentState NodeState

	rw sync.RWMutex
}

func (node *Node) SetEletionTimeout() {

	var min, max int = 3001, 50000
	rand.Seed(time.Now().UnixNano())
	node.eletionTimeoutInterval = rand.Intn(max-min) + min
	node.ElectionTimeoutTicker = time.NewTicker(time.Duration(node.eletionTimeoutInterval) * time.Millisecond)
}

func (node *Node) SetHeartbeatTimeout() {
	node.HeartbeatTimeoutInterval = 2000
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

			fmt.Println("I'm a candidate now")

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
	//todo
	// start new election term
	//1 ccreate new term
	// vote self
	// request votes to perrs

}

func (node *Node) startNewElectionTerm() {
	//todo
	//1 ccreate new term
	// vote self

}

func NewNode() *Node {
	node := &Node{}
	node.SetEletionTimeout()
	node.SetHeartbeatTimeout()
	return node
}
