// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package merkledb

import (
	"math"

	"github.com/ava-labs/avalanchego/ids"
)

const (
	visitNodeNextKey = -1
	firstKeyNotFound = -1
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

	// For each node in the proof, find the next child index to visit.
	var (
		foundEligibleStartKey    bool
		smallestEligibleStartKey path
		firstNodeIndex           = math.MaxInt
	)

	for nodeIndex := 0; nodeIndex < len(proof); nodeIndex++ {
		node := proof[nodeIndex]
		nodePath := iter.nodeToPath[nodeIndex]

		if start.Compare(nodePath) <= 0 {
			// [start] is at/before this node, and therefore all of its descendants.
			// All of them should be iterated over.
			iter.nextChildIndex[nodeIndex] = visitNodeNextKey

			// If this is the smallest eligible start key, set node index.
			if !foundEligibleStartKey || nodePath.Compare(smallestEligibleStartKey) < 0 {
				smallestEligibleStartKey = nodePath
				firstNodeIndex = nodeIndex
				foundEligibleStartKey = true
			}

			continue
		}

		// [start] is after this node. Find which children, if any,
		// we should iterate over.
		for childIdx := byte(0); childIdx < NodeBranchFactor; childIdx++ {
			if _, ok := node.Children[childIdx]; !ok {
				// This child doesn't exist.
				iter.nextChildIndex[nodeIndex] = int(childIdx) + 1
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

			if comp := start.Compare(childKey); comp > 0 {
				// The child is before [start]. Don't iterate over it.
				iter.nextChildIndex[nodeIndex] = int(childIdx) + 1
			} else {
				// The child is at/after [start]. We should iterate over it
				// (and, if it's a node, all its descendants.)
				if !childIsNode {
					if !foundEligibleStartKey || childKey.Compare(smallestEligibleStartKey) < 0 {
						smallestEligibleStartKey = childKey
						firstNodeIndex = nodeIndex
						foundEligibleStartKey = true
					}
					iter.nextChildIndex[nodeIndex] = int(childIdx)
				} else {
					// TODO reduce duplicated code
					if !foundEligibleStartKey || childKey.Compare(smallestEligibleStartKey) < 0 {
						smallestEligibleStartKey = childKey
						firstNodeIndex = nodeIndex + 1
						foundEligibleStartKey = true
					}
					// When we visit [node], we should visit the child
					// after this one.
					nextChildIndex := NodeBranchFactor
					for j := childIdx + 1; j <= NodeBranchFactor; j++ {
						if _, ok := node.Children[j]; ok {
							nextChildIndex = int(j)
							break
						}
					}
					iter.nextChildIndex[nodeIndex] = nextChildIndex
					iter.nextChildIndex[nodeIndex+1] = visitNodeNextKey
				}
				// We don't need to look at any more children.
				break
			}
		}
	}
	iter.nodeIndex = firstNodeIndex
	if firstNodeIndex == math.MaxInt {
		// All keys are after [start].
		iter.exhausted = true
	}
	return iter
}

func (i *proofIterator) Next() bool {
	if i.exhausted {
		i.key = EmptyPath
		i.value = ids.Empty
		return false
	}

	node := i.proof[i.nodeIndex]
	childIdx := i.nextChildIndex[i.nodeIndex]
	shouldVisitNode := childIdx == visitNodeNextKey

	if shouldVisitNode {
		// The node itself should be visited next.
		i.key = i.nodeToPath[i.nodeIndex]
		i.value = i.nodeToID[i.nodeIndex]
	} else {
		i.key = i.nodeToPath[i.nodeIndex].Append(byte(childIdx))
		i.value = node.Children[byte(childIdx)]
	}

	// Find the next child index to visit for this node.
	if shouldVisitNode {
		// We just visited this node. Next time we visit it,
		// we should visit the first child.
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

	// If the next child is in the proof, visit it next.
	if i.nodeIndex != len(i.proof)-1 &&
		i.nodeToBranchIndex[i.nodeIndex] == byte(nextChildIndex) &&
		len(i.proof[i.nodeIndex+1].Children) > 0 {
		// Since we'll visit the child node next,
		// mark that we don't need to visit [node]'s child
		// at [nextChildIndex] when we visit [node] again.
		newNextChildIndex := NodeBranchFactor
		for j := byte(nextChildIndex) + 1; j <= NodeBranchFactor; j++ {
			if _, ok := node.Children[j]; ok {
				newNextChildIndex = int(j)
				break
			}
		}
		i.nextChildIndex[i.nodeIndex] = newNextChildIndex
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
