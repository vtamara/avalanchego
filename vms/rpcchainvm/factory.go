// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package rpcchainvm

import (
	"context"
	"fmt"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/resource"
	"github.com/ava-labs/avalanchego/vms"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm/grpcutils"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm/runtime"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm/runtime/subprocess"
	"go.uber.org/zap"

	vmpb "github.com/ava-labs/avalanchego/proto/pb/vm"
)

var _ vms.Factory = (*factory)(nil)

type factory struct {
	path           string
	processTracker resource.ProcessTracker
	runtimeTracker runtime.Tracker
}

func NewFactory(path string, processTracker resource.ProcessTracker, runtimeTracker runtime.Tracker) vms.Factory {
	return &factory{
		path:           path,
		processTracker: processTracker,
		runtimeTracker: runtimeTracker,
	}
}

func (f *factory) New(log logging.Logger) (interface{}, error) {
	config := &subprocess.Config{
		Stderr:           log,
		Stdout:           log,
		HandshakeTimeout: runtime.DefaultHandshakeTimeout,
		Log:              log,
	}

	listener, err := grpcutils.NewListener()
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %w", err)
	}

	config.Log.Info("starting Bootstrap")
	status, stopper, err := subprocess.Bootstrap(
		context.TODO(),
		listener,
		subprocess.NewCmd(f.path),
		config,
	)
	if err != nil {
		config.Log.Info("Bootstrap returned error", zap.Error(err))
		return nil, err
	}

	config.Log.Info("Bootstrap returned status", zap.Any("status", status))
	clientConn, err := grpcutils.Dial(status.Addr)
	if err != nil {
		config.Log.Info("grpcutils.Dial returned error", zap.Error(err))
		return nil, err
	}
	config.Log.Info("grpcutils.Dial returned nil")

	vm := NewClient(vmpb.NewVMClient(clientConn))
	vm.SetProcess(stopper, status.Pid, f.processTracker)

	f.runtimeTracker.TrackRuntime(stopper)

	config.Log.Info("factory New returned")

	return vm, nil
}
