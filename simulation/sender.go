package simulation

import (
	"context"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/utils/sampler"
	"github.com/ava-labs/avalanchego/utils/set"
)

var _ common.AppSender = (*sender)(nil)

// only supports application messages currently
type sender struct {
	nodeID              ids.NodeID
	appGossipSampleSize int
	network             *network
}

func (s sender) SendCrossChainAppRequest(ctx context.Context, chainID ids.ID, requestID uint32, appRequestBytes []byte) error {
	panic("not implemented yet")
}

func (s sender) SendCrossChainAppResponse(ctx context.Context, chainID ids.ID, requestID uint32, appResponseBytes []byte) error {
	panic("not implemented yet")
}

func (s sender) SendAppRequest(ctx context.Context, nodeIDs set.Set[ids.NodeID], requestID uint32, appRequestBytes []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	for nodeID := range nodeIDs {
		s.send(nodeID, appRequest, appRequestBytes, requestID)
	}

	return nil
}

func (s sender) SendAppResponse(ctx context.Context, nodeID ids.NodeID, requestID uint32, appResponseBytes []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.send(nodeID, appResponse, appResponseBytes, requestID)

	return nil
}

func (s sender) SendAppGossip(ctx context.Context, appGossipBytes []byte) error {
	nodeIDs := set.NewSet[ids.NodeID](s.appGossipSampleSize)

	uniform := sampler.NewUniform()
	uniform.Initialize(uint64(len(s.network.nodes)))

	for i := 0; i < s.appGossipSampleSize; i++ {
		drawn, err := uniform.Next()
		if err != nil {
			return err
		}

		nodeIDs.Add(s.network.nodes[drawn].nodeID)
	}

	return s.SendAppGossipSpecific(ctx, nodeIDs, appGossipBytes)
}

func (s sender) SendAppGossipSpecific(ctx context.Context, nodeIDs set.Set[ids.NodeID], appGossipBytes []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	for nodeID := range nodeIDs {
		s.send(nodeID, appGossip, appGossipBytes, 0)
	}

	return nil
}

func (s sender) send(nodeID ids.NodeID, kind kind, bytes []byte, requestID uint32) {
	if nodeID == s.nodeID {
		// drop messages to myself
		return
	}

	s.network.nodes[s.network.nodeIDsToIndices[nodeID]].incoming <- message{
		from:      s.nodeID,
		kind:      kind,
		bytes:     bytes,
		requestID: requestID,
	}
}

type network struct {
	nodeIDsToIndices map[ids.NodeID]int
	nodes            []Node
}
