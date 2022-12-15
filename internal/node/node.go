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

type Node struct {
	id            string
	leaderNodeId  string
	currentLeader bool

	state NodeState
	peers []string

	electionTimeoutTickerReset chan struct{}
	electionTimeoutTicker      *time.Ticker
	heartbeatTimeoutTicker     *time.Ticker

	eletionTimeoutInterval   int
	heartbeatTimeoutInterval int
	currentTerm              int
	lastCandidateVoted       string

	rw sync.RWMutex
}

func (node *Node) Close() {
	close(node.electionTimeoutTickerReset)
	node.electionTimeoutTicker.Stop()
	node.heartbeatTimeoutTicker.Stop()
}

func (node *Node) setNodeState(state NodeState) {
	defer node.rw.Unlock()
	node.rw.Lock()
	node.state = state
}

func (node *Node) resetElectionTimeoutTicker() {
	defer node.rw.Unlock()
	node.rw.Lock()
	node.electionTimeoutTicker.Reset(time.Duration(node.eletionTimeoutInterval) * time.Millisecond)
}

func (node *Node) becomeLeader() {

	node.currentTerm += 1
	node.setNodeState(Candidate)
	node.resetElectionTimeoutTicker()
	voteRequest := buildVoteRequest(node.currentTerm, node.id, 0, 0)

	log.Printf("[NODE:%s] is CANDIDATE and will try become leader at TERM: %d \n", node.id, node.currentTerm)

	var votesReceived int
	var wg sync.WaitGroup

	for i := range node.peers {

		wg.Add(1)

		go func(voteRequest string, peer string) {

			defer wg.Done()

			log.Printf("[TERM:%d] send vote request to peer: %s \n", node.currentTerm, peer)

			data, err := requestVoteToPeer(peer, voteRequest)
			if err != nil {
				log.Printf("[TERM:%d] failed to request vote to peer:%s - error: %s \n", node.currentTerm, peer, err.Error())
				return
			}

			log.Printf("[TERM:%d] received vote from peer:%s - result: %s \n", node.currentTerm, peer, data)

			term, nodeId, voteResult, err := responseFromVoteRequest(data)
			if err != nil {
				log.Printf("[TERM:%d] invalid response received from peer:%s - err: %s \n", node.currentTerm, peer, err.Error())
				return
			}

			log.Printf("[TERM:%d] The NODE-ID:%s voted:%v at term:%d \n", node.currentTerm, nodeId, voteResult, term)

			if term > node.currentTerm {
				if node.state != Leader {
					node.resetElectionTimeoutTicker()
				}
				node.setNodeState(Follower)
				node.currentTerm = term
				return

			}

			if node.state == Candidate && node.currentTerm == term && voteResult {
				votesReceived += 1
			}

		}(voteRequest, node.peers[i])
	}

	wg.Wait()

	if node.state == Leader {
		return
	}

	// I voted for myself and await the consensus of the other peers
	votesReceived += 1
	node.lastCandidateVoted = node.id

	log.Printf("[TERM:%d] total votes received in faivor: %d \n", node.currentTerm, votesReceived)

	if votesReceived >= (len(node.peers)+1)/2 {

		log.Printf("[TERM:%d] Node: %s was elected the new leader \n", node.currentTerm, node.id)
		node.electionTimeoutTicker.Stop()
		node.setNodeState(Leader)
		node.leaderNodeId = node.id
		node.currentLeader = true
		go node.heartbeatTime()

	} else {

		log.Printf("[TERM:%d] Node: %s was not elected the new leader \n", node.currentTerm, node.id)
		node.heartbeatTimeoutTicker.Stop()
		node.setNodeState(Follower)
		node.currentLeader = false
	}
}

func (node *Node) heartbeatTime() {

	log.Printf("[TERM: %d][NODE: %s] start heartbeat proccess time \n", node.currentTerm, node.id)

	node.heartbeatTimeoutTicker = time.NewTicker(time.Duration(node.heartbeatTimeoutInterval) * time.Millisecond)
	for ticker := range node.heartbeatTimeoutTicker.C {

		for i := range node.peers {

			c, err := net.Dial("tcp", node.peers[i])
			if err != nil {
				log.Printf("ERRO NO HEARTBEAT %s \n", ticker.String())
				continue
			}

			fmt.Printf("[TERM:%d][NODE:%s] I'm still the round leader; sending heartbeat to peer: %s \n", node.currentTerm, node.id, node.peers[i])
			c.Write([]byte(fmt.Sprintf("heartbeat|the NODE:%s is a leader at TERM:%d \n", node.id, node.currentTerm)))
		}
	}
}

func (node *Node) ElectionTime() {
	for {
		select {
		case <-node.electionTimeoutTicker.C:

			if node.state == Follower {
				go node.becomeLeader()
			}

		case <-node.electionTimeoutTickerReset:
			node.resetElectionTimeoutTicker()
		}
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

	termReq, err := strconv.Atoi(splits[1])
	if err != nil {
		return
	}

	if node.state == Leader {
		c.Write([]byte(fmt.Sprintf("vote-response|%s|%s|false \n", splits[1], node.id)))
		return
	}

	if termReq > node.currentTerm {

		node.heartbeatTimeoutTicker.Stop()
		node.setNodeState(Follower)
		node.resetElectionTimeoutTicker()
		node.currentTerm = termReq
		node.lastCandidateVoted = ""
		node.currentLeader = false
	}

	var candidateId string = splits[2]
	var voteResponse string

	if termReq == node.currentTerm && (candidateId == node.lastCandidateVoted || len(node.lastCandidateVoted) == 0) {
		node.lastCandidateVoted = candidateId
		voteResponse = "True"
	} else {
		voteResponse = "False"
	}

	var msgResponse string = "vote-response|" + splits[1] + "|" + node.id + "|" + voteResponse + "\n"
	c.Write([]byte(msgResponse))
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

func NewNode(nodeId string, peers []string) *Node {

	var min, max int = 149, 302
	rand.Seed(time.Now().UnixNano())
	timeoutInterval := rand.Intn(max-min) + min

	return &Node{
		id:                         nodeId,
		leaderNodeId:               "",
		lastCandidateVoted:         "",
		peers:                      peers,
		electionTimeoutTickerReset: make(chan struct{}),
		electionTimeoutTicker:      time.NewTicker(time.Duration(timeoutInterval) * time.Millisecond),
		heartbeatTimeoutTicker:     &time.Ticker{},
		state:                      Follower,
		eletionTimeoutInterval:     timeoutInterval,
		heartbeatTimeoutInterval:   100,
		currentTerm:                0,
		rw:                         sync.RWMutex{},
	}
}
