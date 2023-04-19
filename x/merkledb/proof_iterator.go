// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package merkledb

import (
	"github.com/ava-labs/avalanchego/ids"
)

// Iterates over the key prefixes whose existence is proven by the proof.
// For each key prefix, the value is the hash of the node which is the root
// of that subtrie.
// TODO add support for end path.
// TODO handle returning the ID for the root (undefined).
// TODO handle difference between parent's view of the child path
// vs the child's actual path.
type proofIterator struct {
	// The next key to return
	key path
	// The next value to return
	value ids.ID
	// True iff there are more key/ID pairs to return.
	exhausted bool
	// Index of node in [proof] to visit next.
	nodeIndex int
	// Index of node in [proof] --> next child index to visit for that node.
	// If a key isn't in the map, the first child, if any, should be visited next
	// when visiting that node.
	// If a value is [NodeBranchFactor], all children have been visited and we
	// should ascend to the previous node. If there is no previous node, we're done.
	nextChildIndex map[int]int
	proof          []ProofNode
	// Index of node in [proof] --> path of that node.
	nodeToPath map[int]path
	// Index of node in [proof] --> Index of the child
	// of that node which is the next node in the proof.
	nodeToBranchIndex map[int]byte
}

// Assumes len([proof]) > 0.
func newProofIterator(proof []ProofNode, start path) *proofIterator {
	iter := &proofIterator{
		nextChildIndex:    map[int]int{},
		proof:             proof,
		nodeToPath:        map[int]path{},
		nodeToBranchIndex: map[int]byte{},
	}

	// Populate [iter.nodeToPath].
	for i := 0; i < len(proof); i++ {
		iter.nodeToPath[i] = proof[i].KeyPath.deserialize()
	}

	// Populate [iter.nodeToBranchIndex].
	for i := 0; i < len(proof)-1; i++ {
		myPath := iter.nodeToPath[i]
		nextPath := iter.nodeToPath[i+1]
		iter.nodeToBranchIndex[i] = nextPath[len(myPath)]
	}

	for i := 0; i < len(proof); i++ {
		iter.nodeIndex = i
		node := proof[i]
		path := iter.nodeToPath[i]
		if start.Compare(path) <= 0 {
			// The first key to return is the one in [node].
			return iter
		}

		for childIdx := byte(0); childIdx < NodeBranchFactor; childIdx++ {
			if _, ok := node.Children[childIdx]; !ok {
				continue
			}

			childPrefix := path.Append(childIdx)
			if start.Compare(childPrefix) <= 0 {
				// The first key to return is the one at [childIdx].
				iter.nextChildIndex[i] = int(childIdx)
				return iter
			}
		}
		// All the children are after [start].
		iter.nextChildIndex[i] = int(NodeBranchFactor)
	}

	// All keys are after [start].
	iter.exhausted = true
	return iter
}

// TODO implement
func (i *proofIterator) Next() bool {
	if i.exhausted {
		i.key = EmptyPath
		i.value = ids.Empty
		return false
	}

	node := i.proof[i.nodeIndex]
	childIdx := i.nextChildIndex[i.nodeIndex]
	i.key = i.nodeToPath[i.nodeIndex].Append(byte(childIdx))
	i.value = node.Children[byte(childIdx)]

	// We should descend into the next node if:
	// 1. We are not at the last node in the proof.
	// 2. We just returned the child's path/ID.
	// 3. The next node has children.
	descended := false
	if i.nodeIndex != len(i.proof)-1 &&
		i.nodeToBranchIndex[i.nodeIndex] == byte(childIdx) &&
		len(i.proof[i.nodeIndex+1].Children) > 0 {
		if len(i.proof[i.nodeIndex+1].Children) > 0 {
			// Descend into the next node.
			descended = true
			i.nodeIndex++
		}
	}

	// Update the next child index to visit for this node.
	// Use <= j so that if there are no more children,
	// we set the next child index to be [NodeBranchFactor],
	// which indicates that we are done with this node.
	var nextChildIndex int
	for j := childIdx; j <= int(NodeBranchFactor); j++ {
		if _, ok := node.Children[byte(j)]; ok {
			nextChildIndex = j
		}
	}
	i.nextChildIndex[i.nodeIndex] = nextChildIndex
	if !descended && nextChildIndex == int(NodeBranchFactor) {
		if i.nodeIndex == 0 {
			// We are done with the proof.
			i.exhausted = true
		} else {
			// We are done with this node.
			// Ascend to the node above it, unless we just descended.
			i.nodeIndex--
		}
	}

	return true
}

func (i *proofIterator) Key() path {
	return i.key
}

func (i *proofIterator) Value() ids.ID {
	return i.value
}
