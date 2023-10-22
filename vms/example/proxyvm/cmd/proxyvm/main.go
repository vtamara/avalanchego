// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ava-labs/avalanchego/vms/example/proxyvm/cmd/run"
	"github.com/ava-labs/avalanchego/vms/example/proxyvm/cmd/version"
)

func init() {
	cobra.EnablePrefixMatching = true
}

func main() {
	cmd := run.Command()
	cmd.AddCommand(
		version.Command(),
	)
	ctx := context.Background()
	if err := cmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "command failed %v\n", err)
		os.Exit(1)
	}
}
