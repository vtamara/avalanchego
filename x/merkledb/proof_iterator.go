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
	// If a key isn't in the map, the node itself should be visited next.
	// If a value is [NodeBranchFactor], all children have been visited and we
	// should ascend to the previous node. If there is no previous node, we're done.
	nextChildIndex map[int]int
	proof          []ProofNode
	// Index of node in [proof] --> path of that node.
	nodeToPath map[int]path
	// Index of node in [proof] --> Index of the child
	// of that node which is the next node in the proof.
	nodeToBranchIndex map[int]byte
	// Index of node in [proof] --> its ID.
	// Not defined for the root.
	nodeToID map[int]ids.ID
}

// Assumes len([proof]) > 0.
func newProofIterator(proof []ProofNode, start path) *proofIterator {
	iter := &proofIterator{
		nextChildIndex:    map[int]int{},
		proof:             proof,
		nodeToPath:        map[int]path{},
		nodeToBranchIndex: map[int]byte{},
		nodeToID:          map[int]ids.ID{},
	}

	// Populate [iter.nodeToPath].
	for i := 0; i < len(proof); i++ {
		iter.nodeToPath[i] = proof[i].KeyPath.deserialize()
	}

	// Populate [iter.nodeToBranchIndex].
	for i := 0; i < len(proof)-1; i++ {
		myPath := iter.nodeToPath[i]
		nextPath := iter.nodeToPath[i+1]
		childIndex := nextPath[len(myPath)]
		iter.nodeToBranchIndex[i] = childIndex
		iter.nodeToID[i+1] = proof[i].Children[childIndex]
	}

	// Find the first key to return.
	for nodeIndex := 0; nodeIndex < len(proof); nodeIndex++ {
		iter.nodeIndex = nodeIndex
		node := proof[nodeIndex]
		nodePath := iter.nodeToPath[nodeIndex]

		if start.Compare(nodePath) <= 0 {
			// The first key to return is the one in [node].
			return iter
		}

		for childIdx := byte(0); childIdx < NodeBranchFactor; childIdx++ {
			if _, ok := node.Children[childIdx]; !ok {
				continue
			}

			var (
				childKey    path
				childIsNode bool
			)
			if nodeIndex != len(proof)-1 && iter.nodeToBranchIndex[nodeIndex] == childIdx {
				// The child is in the proof.
				childIsNode = true
				childKey = iter.nodeToPath[nodeIndex+1]
			} else {
				// The child is a leaf.
				childKey = nodePath.Append(childIdx)
			}

			if start.Compare(childKey) <= 0 {
				if childIsNode {
					// The key/ID pair of the child node is the
					// first key/ID pair to return.
					iter.nodeIndex++
					// When we visit [node], we should visit the child
					// following the one we just descended to.
					nextChildIndex := NodeBranchFactor
					for j := childIdx + 1; j <= NodeBranchFactor; j++ {
						if _, ok := node.Children[j]; ok {
							nextChildIndex = int(j)
							break
						}
					}
					iter.nextChildIndex[nodeIndex] = nextChildIndex
				} else {
					// The first key to return is the one at [childIdx].
					iter.nextChildIndex[nodeIndex] = int(childIdx)
				}
				return iter
			}
		}
		// All the children are after [start].
		iter.nextChildIndex[nodeIndex] = int(NodeBranchFactor)
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
	childIdx, alreadyVisitedNode := i.nextChildIndex[i.nodeIndex]

	// for childIdx == int(NodeBranchFactor) {
	// 	// We've visited all the children of this node.
	// 	// Ascend to the previous node.
	// 	if i.nodeIndex == 0 {
	// 		// We've visited all the nodes in the proof.
	// 		i.exhausted = true
	// 		i.key = EmptyPath
	// 		i.value = ids.Empty
	// 		return false
	// 	}
	// 	i.nodeIndex--
	// 	childIdx = i.nextChildIndex[i.nodeIndex]
	// }

	if !alreadyVisitedNode {
		// The node itself should be visited next.
		i.key = i.nodeToPath[i.nodeIndex]
		i.value = i.nodeToID[i.nodeIndex]
	} else {
		i.key = i.nodeToPath[i.nodeIndex].Append(byte(childIdx))
		i.value = node.Children[byte(childIdx)]
	}

	// Find the next child index to visit for this node.
	if !alreadyVisitedNode {
		// We just visited this node.
		// In the loop below, start looking from child index 0.
		childIdx = -1
	}
	// Use <= j so that if there are no more children,
	// we set [nextChildIndex] to [NodeBranchFactor],
	// which indicates that we're done with this node.
	nextChildIndex := int(NodeBranchFactor)
	for j := childIdx + 1; j <= int(NodeBranchFactor); j++ {
		if _, ok := node.Children[byte(j)]; ok {
			nextChildIndex = j
			break
		}
	}
	i.nextChildIndex[i.nodeIndex] = nextChildIndex

	// If the next child is in the proof, descend to it.
	if i.nodeIndex != len(i.proof)-1 &&
		i.nodeToBranchIndex[i.nodeIndex] == byte(nextChildIndex) &&
		len(i.proof[i.nodeIndex+1].Children) > 0 {
		i.nodeIndex++
	}

	// If we've visited all the children of this node,
	// ascend to the nearest node that isn't exhausted.
	if nextChildIndex == int(NodeBranchFactor) {
		// Note it's impossible for us to have
		// just descended to the next proof node
		// because there's no branch index at [NodeBranchFactor].
		if i.nodeIndex == 0 {
			// We are done with the proof.
			i.exhausted = true
		} else {
			// We are done with this node.
			// Ascend to the node above it, unless we just descended.
			i.nodeIndex--
			for i.nextChildIndex[i.nodeIndex] == int(NodeBranchFactor) {
				if i.nodeIndex == 0 {
					i.exhausted = true
					break
				}
				i.nodeIndex--
			}
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
