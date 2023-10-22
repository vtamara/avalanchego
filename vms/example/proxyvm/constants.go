// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package proxyvm

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/version"
)

const Name = "proxyvm"

var (
	ID = ids.ID{'p', 'r', 'o', 'x', 'y', 'v', 'm'}

	Version = &version.Semantic{
		Major: 0,
		Minor: 0,
		Patch: 1,
	}
)
