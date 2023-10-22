// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package proxyvm

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/ava-labs/avalanchego/snow/engine/common"

	vmpb "github.com/ava-labs/avalanchego/proto/pb/vm"
)

type InitConfig struct {
	// Address to register VMServer with proxyvm
	Address string
}

type ProxyVM struct {
	common.AppHandler

	client vmpb.VMClient
}

// NewProxyVM returns a vm instance that proxies calls to a remote vm instance
func NewProxyVM(vm block.ChainVM, allowShutdown *utils.Atomic[bool]) *ProxyVM {
	bVM, _ := vm.(block.BuildBlockWithContextChainVM)
	ssVM, _ := vm.(block.StateSyncableVM)
	return &VMServer{
		vm:            vm,
		bVM:           bVM,
		ssVM:          ssVM,
		allowShutdown: allowShutdown,
	}
}

func (vm *ProxyVM) Initialize(ctx context.Context, req *vmpb.InitializeRequest) (*vmpb.InitializeResponse, error) {
	return vm.client.Initialize(ctx, req)
}

func (vm *ProxyVM) SetState(ctx context.Context, stateReq *vmpb.SetStateRequest) (*vmpb.SetStateResponse, error) {
	return vm.client.SetState(ctx, stateReq)
}

func (vm *ProxyVM) Shutdown(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	return vm.client.Shutdown(ctx, empty)
}

func (vm *ProxyVM) CreateHandlers(ctx context.Context, empty *emptypb.Empty) (*vmpb.CreateHandlersResponse, error) {
	return vm.client.CreateHandlers(ctx, empty)
}

func (vm *ProxyVM) CreateStaticHandlers(ctx context.Context, empty *emptypb.Empty) (*vmpb.CreateStaticHandlersResponse, error) {
	return vm.client.CreateStaticHandlers(ctx, empty)
}

func (vm *ProxyVM) Connected(ctx context.Context, req *vmpb.ConnectedRequest) (*emptypb.Empty, error) {
	return vm.client.Connected(ctx, req)
}

func (vm *ProxyVM) Disconnected(ctx context.Context, req *vmpb.DisconnectedRequest) (*emptypb.Empty, error) {
	return vm.client.Disconnected(ctx, req)
}

func (vm *ProxyVM) BuildBlock(ctx context.Context, req *vmpb.BuildBlockRequest) (*vmpb.BuildBlockResponse, error) {
	return vm.client.BuildBlock(ctx, req)
}

func (vm *ProxyVM) ParseBlock(ctx context.Context, req *vmpb.ParseBlockRequest) (*vmpb.ParseBlockResponse, error) {
	return vm.client.ParseBlock(ctx, req)
}

func (vm *ProxyVM) GetBlock(ctx context.Context, req *vmpb.GetBlockRequest) (*vmpb.GetBlockResponse, error) {
	return vm.client.GetBlock(ctx, req)
}

func (vm *ProxyVM) SetPreference(ctx context.Context, req *vmpb.SetPreferenceRequest) (*emptypb.Empty, error) {
	return vm.client.SetPreference(ctx, req)
}

func (vm *ProxyVM) Health(ctx context.Context, empty *emptypb.Empty) (*vmpb.HealthResponse, error) {
	return vm.client.Health(ctx, empty)
}

func (vm *ProxyVM) Version(ctx context.Context, empty *emptypb.Empty) (*vmpb.VersionResponse, error) {
	return vm.client.Version(ctx, empty)
}

func (vm *ProxyVM) CrossChainAppRequest(ctx context.Context, msg *vmpb.CrossChainAppRequestMsg) (*emptypb.Empty, error) {
	return vm.client.CrossChainAppRequest(ctx, msg)
}

func (vm *ProxyVM) CrossChainAppRequestFailed(ctx context.Context, msg *vmpb.CrossChainAppRequestFailedMsg) (*emptypb.Empty, error) {
	return vm.client.CrossChainAppRequestFailed(ctx, msg)
}

func (vm *ProxyVM) CrossChainAppResponse(ctx context.Context, msg *vmpb.CrossChainAppResponseMsg) (*emptypb.Empty, error) {
	return vm.client.CrossChainAppResponse(ctx, msg)
}

func (vm *ProxyVM) AppRequest(ctx context.Context, req *vmpb.AppRequestMsg) (*emptypb.Empty, error) {
	return vm.client.AppRequest(ctx, req)
}

func (vm *ProxyVM) AppRequestFailed(ctx context.Context, req *vmpb.AppRequestFailedMsg) (*emptypb.Empty, error) {
	return vm.client.AppRequestFailed(ctx, req)
}

func (vm *ProxyVM) AppResponse(ctx context.Context, req *vmpb.AppResponseMsg) (*emptypb.Empty, error) {
	return vm.client.AppResponse(ctx, req)
}

func (vm *ProxyVM) AppGossip(ctx context.Context, req *vmpb.AppGossipMsg) (*emptypb.Empty, error) {
	return vm.client.AppGossip(ctx, req)
}

func (vm *ProxyVM) Gather(ctx context.Context, empty *emptypb.Empty) (*vmpb.GatherResponse, error) {
	return vm.client.Gather(ctx, empty)
}

func (vm *ProxyVM) GetAncestors(ctx context.Context, req *vmpb.GetAncestorsRequest) (*vmpb.GetAncestorsResponse, error) {
	return vm.client.GetAncestors(ctx, req)
}

func (vm *ProxyVM) BatchedParseBlock(
	ctx context.Context,
	req *vmpb.BatchedParseBlockRequest,
) (*vmpb.BatchedParseBlockResponse, error) {
	return vm.client.BatchedParseBlock(ctx, req)
}

func (vm *ProxyVM) VerifyHeightIndex(ctx context.Context, empty *emptypb.Empty) (*vmpb.VerifyHeightIndexResponse, error) {
	return vm.client.VerifyHeightIndex(ctx, empty)
}

func (vm *ProxyVM) GetBlockIDAtHeight(
	ctx context.Context,
	req *vmpb.GetBlockIDAtHeightRequest,
) (*vmpb.GetBlockIDAtHeightResponse, error) {
	return vm.client.GetBlockIDAtHeight(ctx, req)
}

func (vm *ProxyVM) StateSyncEnabled(ctx context.Context, empty *emptypb.Empty) (*vmpb.StateSyncEnabledResponse, error) {
	return vm.client.StateSyncEnabled(ctx, empty)
}

func (vm *ProxyVM) GetOngoingSyncStateSummary(
	ctx context.Context,
	empty *emptypb.Empty,
) (*vmpb.GetOngoingSyncStateSummaryResponse, error) {
	return vm.client.GetOngoingSyncStateSummary(ctx, empty)
}

func (vm *ProxyVM) GetLastStateSummary(ctx context.Context, empty *emptypb.Empty) (*vmpb.GetLastStateSummaryResponse, error) {
	return vm.client.GetLastStateSummary(ctx, empty)
}

func (vm *ProxyVM) ParseStateSummary(
	ctx context.Context,
	req *vmpb.ParseStateSummaryRequest,
) (*vmpb.ParseStateSummaryResponse, error) {
	return vm.client.ParseStateSummary(ctx, req)
}

func (vm *ProxyVM) GetStateSummary(
	ctx context.Context,
	req *vmpb.GetStateSummaryRequest,
) (*vmpb.GetStateSummaryResponse, error) {
	return vm.client.GetStateSummary(ctx, req)
}

func (vm *ProxyVM) BlockVerify(ctx context.Context, req *vmpb.BlockVerifyRequest) (*vmpb.BlockVerifyResponse, error) {
	return vm.client.BlockVerify(ctx, req)
}

func (vm *ProxyVM) BlockAccept(ctx context.Context, req *vmpb.BlockAcceptRequest) (*emptypb.Empty, error) {
	return vm.client.BlockAccept(ctx, req)
}

func (vm *ProxyVM) BlockReject(ctx context.Context, req *vmpb.BlockRejectRequest) (*emptypb.Empty, error) {
	return vm.client.BlockReject(ctx, req)
}

func (vm *ProxyVM) StateSummaryAccept(
	ctx context.Context,
	req *vmpb.StateSummaryAcceptRequest,
) (*vmpb.StateSummaryAcceptResponse, error) {
	return vm.client.StateSummaryAccept(ctx, req)
}
