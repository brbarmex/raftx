package node

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
)

func buildVoteRequest(term int, candidateId string, lastLogIndex, lastLogTerm int) string {
	return fmt.Sprintf("vote-request|%d|%s|%d|%d\n", term, candidateId, lastLogIndex, lastLogTerm)
}

func requestVoteToPeer(peer, request string) (string, error) {

	c, err := net.Dial("tcp", peer)
	if err != nil {
		log.Println(err)
		return "", err
	}

	defer c.Close()

	c.Write([]byte(request))
	return bufio.NewReader(c).ReadString('\n')
}

func responseFromVoteRequest(message string) (term int, nodeId string, voteResult bool, err error) {

	split := strings.Split(strings.TrimSpace(string(message)), "|")
	if len(split) < 4 ||
		len(split[0]) == 0 ||
		len(split[1]) == 0 ||
		len(split[2]) == 0 {
		err = fmt.Errorf("invalid response from vote request: %s \n", message)
		return
	}

	term, err = strconv.Atoi(split[1])
	if err != nil {
		return
	}

	voteResult, err = strconv.ParseBool(split[3])
	if err != nil {
		return
	}

	nodeId = split[2]
	if len(nodeId) == 0 {
		err = fmt.Errorf("response from vote request contains a node-id invalid. %s \n", message)
		return
	}

	return
}
