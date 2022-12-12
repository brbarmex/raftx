package main

import (
	"github.com/brbarmex/raftx/internal/server"
	"github.com/brbarmex/raftx/pkg/config"
)

func main() {

	cfg := config.NewConfig()

	server := server.NewServer(cfg)

	server.Start()

}
