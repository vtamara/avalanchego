// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package merkledb

import (
	"github.com/ava-labs/avalanchego/ids"
)

// Iterates over the key prefixes whose existence is proven by the proof.
// For each key prefix, the value is the hash of the node which is the root
// of that subtrie.
type proofIterator struct {
	nodeIndex int
	// Index in [proof] --> next child index to visit for that node.
	// No key exists for a node --> we haven't visited any of its children.
	childIndices map[int]int
	proof        []ProofNode
	end          path
	exhausted    bool
}

// Assumes len([proof]) > 0.
// TODO add support for end path.
func newProofIterator(proof []ProofNode, start path) *proofIterator {
	iter := &proofIterator{
		childIndices: map[int]int{},
		proof:        proof,
	}

	for i := 0; i < len(proof); i++ {
		iter.nodeIndex = i
		node := proof[i]
		path := node.KeyPath.deserialize()
		if start.Compare(path) <= 0 {
			// The first key to return is the one in [node].
			return iter
		}

		for childIdx := byte(0); childIdx < NodeBranchFactor; childIdx++ {
			if _, ok := node.Children[childIdx]; !ok {
				// Mark that we don't need to iterate over this child.
				iter.childIndices[i] = int(childIdx) + 1
				continue
			}

			childPrefix := path.Append(childIdx)
			if start.Compare(childPrefix) <= 0 {
				// The first key to return is the one at [childIdx].
				iter.childIndices[i] = int(childIdx)
				return iter
			}
		}
	}

	// All keys are after [start].
	iter.exhausted = true
	return iter
}

// TODO implement
func (i *proofIterator) Next() bool {
	return false
}

// TODO implement
func (i *proofIterator) Key() []byte {
	return nil
}

// TODO implement
func (i *proofIterator) Value() ids.ID {
	return ids.ID{}
}
