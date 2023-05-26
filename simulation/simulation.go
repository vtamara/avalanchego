package simulation

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/vms"
)

func New(
	ctx context.Context,
	vmFactory vms.Factory,
	numberOfNodes int,
	nodeConfig NodeConfig,
	senderConfig SenderConfig,
	genesisBytes []byte,
	configBytes []byte,
) (Simulation, error) {
	ctx, cancel := context.WithCancel(ctx)
	network := &network{
		nodes:            make([]Node, 0, numberOfNodes),
		nodeIDsToIndices: make(map[ids.NodeID]int, numberOfNodes),
	}
	for i := 0; i < numberOfNodes; i++ {
		nodeID := ids.GenerateTestNodeID()
		sender := sender{
			nodeID:              nodeID,
			appGossipSampleSize: senderConfig.AppGossipSize,
			network:             network,
		}
		node, err := newNode(ctx, sender, vmFactory, nodeID, nodeConfig, genesisBytes, configBytes)
		if err != nil {
			cancel()
			return Simulation{}, err
		}

		network.nodeIDsToIndices[node.nodeID] = len(network.nodes)
		network.nodes = append(network.nodes, node)
	}

	return Simulation{
		ctx:    ctx,
		cancel: cancel,
		nodes:  network.nodes,
	}, nil
}

type Simulation struct {
	ctx    context.Context
	cancel context.CancelFunc

	vmFactory vms.Factory
	nodes     []Node
}

func (s Simulation) Start() {
	for _, node := range s.nodes {
		go node.readMessages()
	}
	for _, node := range s.nodes {
		node.startVM()
	}
}

func (s Simulation) Stop() {
	s.cancel()
}

func (s Simulation) Get(i int) Node {
	return s.nodes[i]
}
