package main

import (
	"github.com/brbarmex/raftx/server"
)

func main() {

	server := server.NewServer()

	server.Start()

}
