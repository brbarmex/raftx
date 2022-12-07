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
	id           string
	leaderNodeId string

	Peers [1]string

	electionTimeoutTicker  *time.Ticker
	heartbeatTimeoutTicker *time.Ticker

	resetElectionTimeoutChn chan struct{}

	currentNodeState         NodeState
	eletionTimeoutInterval   int
	heartbeatTimeoutInterval int
	IdleTimeout              int
	currentTerm              int
	lastCandidateVoted       string

	rw sync.RWMutex
}

func (node *Node) SetEletionTimeout() {

	var min, max int = 149, 302
	rand.Seed(time.Now().UnixNano())
	node.eletionTimeoutInterval = rand.Intn(max-min) + min
	node.electionTimeoutTicker = time.NewTicker(time.Duration(node.eletionTimeoutInterval) * time.Millisecond)
}

func (node *Node) SetHeartbeatTimeout() {
	node.heartbeatTimeoutInterval = 100
}

func (node *Node) setNodeState(state NodeState) {

	defer node.rw.Unlock()
	node.rw.Lock()
	node.currentNodeState = state
}

func (node *Node) resetElectionTimeoutTicker() {

	defer node.rw.Unlock()
	node.rw.Lock()

	if node.electionTimeoutTicker != nil {
		node.electionTimeoutTicker.Reset(time.Duration(node.eletionTimeoutInterval) * time.Millisecond)
	}

}

func (node *Node) ElectionTime() {
	for {
		select {
		case <-node.electionTimeoutTicker.C:

			if node.currentNodeState == Follower {
				go node.becomeLeader()
			}

		case <-node.resetElectionTimeoutChn:
			node.resetElectionTimeoutTicker()
		}
	}
}

func (node *Node) becomeLeader() {

	node.currentTerm += 1
	node.setNodeState(Candidate)
	node.resetElectionTimeoutTicker()
	voteRequest := node.buildVoteRequest(node.currentTerm, node.id, 0, 0)

	var totalVotesReceivedInFavor int
	var wg sync.WaitGroup

	for i := range node.Peers {

		wg.Add(i + 1)

		go func(msgReq string, peer string) {

			defer wg.Done()

			c, err := net.Dial("tcp", peer)
			if err != nil {
				log.Println(err)
				return
			}

			defer c.Close()

			log.Printf("[TERM:%d] sending request vote to peer: %s \n", node.currentTerm, peer)
			c.Write([]byte(msgReq + "\n"))

			data, err := bufio.NewReader(c).ReadString('\n')
			if err != nil {

				log.Printf("[TERM:%d] failed to send request to peer %s; error: %s \n", node.currentTerm, peer, err.Error())
				return
			}

			log.Printf("[TERM:%d] received message from peer %s; message: %s \n", node.currentTerm, peer, data)

			splitRequest := strings.Split(strings.TrimSpace(string(data)), "|")
			if len(splitRequest) < 3 ||
				len(splitRequest[0]) == 0 ||
				len(splitRequest[1]) == 0 ||
				len(splitRequest[2]) == 0 {
				return
			}

			term, err := strconv.Atoi(splitRequest[2])
			if err != nil {
				return
			}

			voteResult, err := strconv.ParseBool(splitRequest[3])
			if err != nil {
				return
			}

			if term > node.currentTerm {
				if node.currentNodeState != Leader {
					node.resetElectionTimeoutTicker()
				}
				node.setNodeState(Follower)
				node.currentTerm = term
				return

			}

			if node.currentNodeState == Candidate && node.currentTerm == term && voteResult {
				totalVotesReceivedInFavor += 1
			}

		}(voteRequest, node.Peers[i])
	}

	wg.Wait()

	if node.currentNodeState == Leader {
		return
	}

	// I voted for myself and await the consensus of the other peers
	totalVotesReceivedInFavor += 1

	log.Printf("[TERM:%d] total votes received in faivor: %d \n", node.currentTerm, totalVotesReceivedInFavor)

	if totalVotesReceivedInFavor >= (len(node.Peers)+1)/2 {

		log.Printf("[TERM:%d] Node: %s was elected the new leader \n", node.currentTerm, node.id)
		node.electionTimeoutTicker.Stop()
		node.setNodeState(Leader)
		node.leaderNodeId = node.id
		go node.heartbeatTime()

	} else {

		log.Printf("[TERM:%d] Node: %s was not elected the new leader \n", node.currentTerm, node.id)
		if node.heartbeatTimeoutTicker != nil {
			node.heartbeatTimeoutTicker.Stop()
		}

		node.setNodeState(Follower)
	}

}

func (node *Node) heartbeatTime() {

	node.heartbeatTimeoutTicker = time.NewTicker(time.Duration(node.heartbeatTimeoutInterval) * time.Millisecond)
	for ticker := range node.heartbeatTimeoutTicker.C {

		for i := range node.Peers {

			c, err := net.Dial("tcp", node.Peers[i])
			if err != nil {
				log.Printf("ERRO NO HEARTBEAT %s \n", ticker.String())
				continue
			}

			fmt.Printf("sending heartbeat to peer: %s \n", node.Peers[i])
			c.Write([]byte("heartbeat" + "\n"))
		}

	}

}

func (node *Node) HandlerRequest(c net.Conn) {

	msg, err := bufio.NewReader(c).ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) {
			return
		}

		log.Println("nao consigo ler", err)
		return
	}

	result := strings.Split(strings.TrimSpace(msg), "|")[0]
	switch result {
	case "vote-request":
		node.voteRequestUseCase(strings.TrimSpace(string(msg)), c)
	case "heartbeat":
		log.Println("received signal heartbeat")
		node.setNodeState(Follower)
		node.resetElectionTimeoutTicker()
		node.currentTerm = 0
		node.lastCandidateVoted = ""
	default:
		log.Println(msg)
		c.Close()
	}

}

func (node *Node) voteRequestUseCase(messageReceived string, c net.Conn) {

	defer c.Close()

	log.Println(messageReceived)

	splits := strings.Split(messageReceived, "|")
	if len(splits) < 3 || len(splits[0]) == 0 || len(splits[1]) == 0 || len(splits[2]) == 0 {

		log.Println(messageReceived)
		// traitment here
		return
	}

	termReq, err := strconv.Atoi(splits[2])
	if err != nil {
		return
	}

	if node.currentNodeState == Leader {
		c.Write([]byte(fmt.Sprintf("vote-response|%s|%s|false \n", splits[2], node.id)))
		return
	}

	if termReq > node.currentTerm {

		if node.heartbeatTimeoutTicker != nil {
			node.heartbeatTimeoutTicker.Stop()
		}

		node.setNodeState(Follower)
		node.resetElectionTimeoutTicker()
		node.currentTerm = termReq
		node.lastCandidateVoted = ""
	}

	var candidateId string = splits[3]
	var voteResponse string

	if termReq == node.currentTerm && (candidateId == node.lastCandidateVoted || len(node.lastCandidateVoted) == 0) {
		node.lastCandidateVoted = candidateId
		voteResponse = "True"
	} else {
		voteResponse = "False"
	}

	var msgResponse string = "vote-response|" + splits[2] + "|" + node.id + "|" + voteResponse + "\n"
	c.Write([]byte(msgResponse))
}

func (node *Node) Close() {

	close(node.resetElectionTimeoutChn)
	node.electionTimeoutTicker.Stop()
	if node.heartbeatTimeoutTicker != nil {
		node.heartbeatTimeoutTicker.Stop()
	}

}

func (node *Node) buildVoteRequest(term int, candidateId string, lastLogIndex, lastLogTerm int) string {
	return fmt.Sprintf("vote-request|%d|%s|%d|%d", term, candidateId, lastLogIndex, lastLogTerm)
}

func NewNode(nodeId string, peers []string) *Node {
	node := &Node{}
	node.id = nodeId
	node.setNodeState(Follower)
	node.SetEletionTimeout()
	node.SetHeartbeatTimeout()
	return node
}
