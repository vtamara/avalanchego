// Copyright (C) 2019-2022, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package subprocess

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm/grpcutils"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm/gruntime"
	"github.com/ava-labs/avalanchego/vms/rpcchainvm/runtime"

	pb "github.com/ava-labs/avalanchego/proto/pb/vm/runtime"
)

type Config struct {
	// Stderr of the VM process written to this writer.
	Stderr io.Writer
	// Stdout of the VM process written to this writer.
	Stdout io.Writer
	// Duration engine server will wait for handshake success.
	HandshakeTimeout time.Duration
	Log              logging.Logger
}

type Status struct {
	// Id of the process.
	Pid int
	// Address of the VM gRPC service.
	Addr string
}

// Bootstrap starts a VM as a subprocess after initialization completes and
// pipes the IO to the appropriate writers.
//
// The subprocess is expected to be stopped by the caller if a non-nil error is
// returned. If piping the IO fails then the subprocess will be stopped.
//
// TODO: create the listener inside this method once we refactor the tests
func Bootstrap(
	ctx context.Context,
	listener net.Listener,
	cmd *exec.Cmd,
	config *Config,
) (*Status, runtime.Stopper, error) {
	defer listener.Close()

	switch {
	case cmd == nil:
		return nil, nil, fmt.Errorf("%w: cmd required", runtime.ErrInvalidConfig)
	case config.Log == nil:
		return nil, nil, fmt.Errorf("%w: logger required", runtime.ErrInvalidConfig)
	case config.Stderr == nil, config.Stdout == nil:
		return nil, nil, fmt.Errorf("%w: stderr and stdout required", runtime.ErrInvalidConfig)
	}

	intitializer := newInitializer()

	server := grpcutils.NewServer()
	defer server.GracefulStop()
	pb.RegisterRuntimeServer(server, gruntime.NewServer(intitializer))

	log := config.Log

	fmt.Println("starting server")
	go grpcutils.Serve(listener, server)

	serverAddr := listener.Addr()
	fmt.Printf("server address: %s\n", serverAddr.String())
	cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", runtime.EngineAddressKey, serverAddr.String()))
	// pass golang debug env to subprocess
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "GRPC_") || strings.HasPrefix(env, "GODEBUG") {
			fmt.Printf("passing env to subprocess: %s", env)
			cmd.Env = append(cmd.Env, env)
		}
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// start subproccess
	fmt.Println("starting command")
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("failed to start process: %w", err)
	}
	fmt.Println("command started")

	stopper := NewStopper(log, cmd)

	// start stdout collector
	go func() {
		fmt.Println("started stdout collector")
		_, err := io.Copy(config.Stdout, stdoutPipe)
		if err != nil {
			fmt.Printf("stdout collector failed: %s\n",
				err,
			)
		}
		stopper.Stop(context.TODO())

		fmt.Println("stdout collector shutdown")
	}()

	// start stderr collector
	go func() {
		fmt.Println("started stderr collector")
		_, err := io.Copy(config.Stderr, stderrPipe)
		if err != nil {
			fmt.Printf("stderr collector failed: %s\n", err)
		}
		stopper.Stop(context.TODO())

		fmt.Println("stderr collector shutdown")
	}()

	// wait for handshake success
	timeout := time.NewTimer(config.HandshakeTimeout)
	defer timeout.Stop()

	select {
	case <-intitializer.initialized:
		fmt.Println("initialization done")
	case <-timeout.C:
		fmt.Println("initialization timeout fired")
		stopper.Stop(ctx)
		fmt.Println("stopper done after initialization timeout")
		return nil, nil, fmt.Errorf("%w: %v", runtime.ErrHandshakeFailed, runtime.ErrProcessNotFound)
	}

	if intitializer.err != nil {
		fmt.Printf("initialization error: %s\n", intitializer.err)
		stopper.Stop(ctx)
		fmt.Println("stopper done after initialization error")
		return nil, nil, fmt.Errorf("%w: %v", runtime.ErrHandshakeFailed, err)
	}

	fmt.Printf("plugin handshake succeeded %s\n",
		intitializer.vmAddr,
	)

	status := &Status{
		Pid:  cmd.Process.Pid,
		Addr: intitializer.vmAddr,
	}
	return status, stopper, nil
}
