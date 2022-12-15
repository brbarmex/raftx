package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/brbarmex/raftx/internal/node"
	"github.com/brbarmex/raftx/pkg/config"
	"github.com/brbarmex/raftx/pkg/nettcp"
	"github.com/spf13/viper"
)

var (
	cfg    *viper.Viper
	peers  []string
	nodeId string
	addr   string
)

func init() {

	cfg = config.NewConfig()

	if !cfg.InConfig("peers") {
		log.Fatalln("key PEERS not found in env")
	} else {
		peers = strings.Split(viper.GetString("peers"), ",")
	}

	if !cfg.InConfig("app.address") {
		log.Fatalln("key APP_ADDRESS not found in env")
	} else {
		addr = cfg.GetString("app.address")
	}

	if !cfg.InConfig("app.node-id") {
		log.Fatalln("key APP_NODEID not found in env")
	} else {
		nodeId = cfg.GetString("app.node-id")
	}
}

func main() {

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	node := node.NewNode(nodeId, peers)
	tcp := nettcp.NewTCP(node, addr)

	go node.ElectionTime()
	go tcp.Listener(ctx)

	exit := make(chan os.Signal, 1)
	signal.Notify(exit, syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGTERM)

	<-exit
	cancel()

}
