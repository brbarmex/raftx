package node

type NodeState int

const (
	Follower NodeState = iota + 1
	Candidate
	Leader
)
