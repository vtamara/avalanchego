package simulation

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"golang.org/x/exp/rand"

	"github.com/ava-labs/avalanchego/database/manager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/version"
	"github.com/ava-labs/avalanchego/vms"
)

type kind int

const (
	appRequest  kind = iota
	appResponse kind = iota
	appGossip   kind = iota
)

type message struct {
	from, to  ids.NodeID
	kind      kind
	bytes     []byte
	requestID uint32
}

type SenderConfig struct {
	AppGossipSize int
}

type NodeConfig struct {
	MessageTimeout time.Duration
}

func newNode(
	ctx context.Context,
	sender sender,
	vmFactory vms.Factory,
	nodeID ids.NodeID,
	config NodeConfig,
	genesisBytes []byte,
	configBytes []byte,
) (Node, error) {
	vm, err := vmFactory.New(logging.NoLog{})
	if err != nil {
		return Node{}, err
	}
	chainVM, ok := vm.(block.ChainVM)
	if !ok {
		return Node{}, errors.New("not a chain vm")
	}

	snowCtx := snow.DefaultContextTest()
	snowCtx.NodeID = ids.GenerateTestNodeID()

	dbManager := manager.NewMemDB(&version.Semantic{
		Major: 0,
		Minor: 0,
		Patch: 0,
	})

	toConsensus := make(chan common.Message)
	// constantly drain consensus channel
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}

			<-toConsensus
		}
	}()

	if err := chainVM.Initialize(
		ctx,
		snowCtx,
		dbManager,
		genesisBytes,
		nil,
		configBytes,
		toConsensus,
		nil,
		sender,
	); err != nil {
		return Node{}, err
	}

	n := Node{
		ctx:            ctx,
		log:            logging.NoLog{},
		sender:         sender,
		vm:             chainVM,
		nodeID:         nodeID,
		incoming:       make(chan message, 10),
		messageTimeout: config.MessageTimeout,
	}

	return n, nil
}

type Node struct {
	ctx              context.Context
	log              logging.Logger
	sender           sender
	vm               block.ChainVM
	nodeID           ids.NodeID
	nodeIDsToIndices map[ids.NodeID]int
	incoming         chan message
	messageTimeout   time.Duration
}

func (n Node) GetVM() block.ChainVM {
	return n.vm
}

func (n Node) startVM() {
	if err := n.vm.SetState(n.ctx, snow.NormalOp); err != nil {
		panic(err)
	}
}

func (n Node) readMessages() {
	for {
		select {
		case <-n.ctx.Done():
			return
		case message := <-n.incoming:
			r := rand.Intn(100)
			time.Sleep(time.Duration(r) * time.Millisecond)

			n.handle(message)
		}
	}
}

func (n Node) handle(message message) {
	switch message.kind {
	case appRequest:
		if err := n.vm.AppRequest(
			n.ctx,
			message.from,
			message.requestID,
			time.Now().Add(n.messageTimeout),
			message.bytes,
		); err != nil {
			n.log.Error("app request failed", zap.Error(err))
		}
	case appResponse:
		if err := n.vm.AppResponse(
			n.ctx,
			message.from,
			message.requestID,
			message.bytes,
		); err != nil {
			n.log.Error("app response failed", zap.Error(err))
		}
	case appGossip:
		if err := n.vm.AppGossip(
			n.ctx,
			message.from,
			message.bytes,
		); err != nil {
			n.log.Error("app gossip failed", zap.Error(err))
		}
	default:
		panic("unexpected message type")
	}
}
