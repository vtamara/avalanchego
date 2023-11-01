// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package merkledb

import (
	"golang.org/x/exp/slices"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/utils/maybe"
)

const HashLength = 32

type nodeChildren map[byte]*child
type child struct {
	compressedKey Key
	id            ids.ID
}

// Returns and caches the ID of this node.
func calculateID(metrics merkleMetrics, key Key, children map[byte]*child, value maybe.Maybe[[]byte]) ids.ID {
	valueDigest := value
	if value.HasValue() && len(value.Value()) >= HashLength {
		valueDigest = maybe.Some(hashing.ComputeHash256(value.Value()))
	}
	metrics.HashCalculated()
	bytes := codec.encodeHashValues(key, children, valueDigest)
	return hashing.ComputeHash256Array(bytes)
}

// Returns the ProofNode representation of this node.
func asProofNode(key Key, children map[byte]*child, value maybe.Maybe[[]byte]) ProofNode {
	var valueDigest maybe.Maybe[[]byte]
	if value.HasValue() && len(value.Value()) >= HashLength {
		valueDigest = maybe.Some(hashing.ComputeHash256(value.Value()))
	} else {
		valueDigest = maybe.Bind(value, slices.Clone[[]byte])
	}

	pn := ProofNode{
		Key:         key,
		Children:    make(map[byte]ids.ID, len(children)),
		ValueOrHash: valueDigest,
	}
	for index, entry := range children {
		pn.Children[index] = entry.id
	}
	return pn
}
