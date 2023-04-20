// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package merkledb

import (
	"math"

	"github.com/ava-labs/avalanchego/ids"
)

const (
	visitNodeNextVal         = -1
	nextNodeIndexNotFoundVal = math.MaxInt
)

// Iterates over the key prefixes whose existence is proven by the proof.
// For each key prefix, the value is the hash of the node which is the root
// of that subtrie.
// TODO add support for end path.
type proofIterator struct {
	// The next key to return
	key path
	// The next value to return
	value ids.ID
	// True iff there are more key/ID pairs to return.
	exhausted bool
	// Index of node in [proof] to visit next.
	nextNodeIndex int
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
		nextNodeIndex:     nextNodeIndexNotFoundVal,
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

	var (
		smallestEligibleStartKey path
		foundEligibleStartKey    bool
	)

	// For each node in the proof, find the next child index to visit.
	// Note that all keys in a proof node are after the key of the previous proof node.
	for nodeIndex := 0; nodeIndex < len(proof); nodeIndex++ {
		node := proof[nodeIndex]
		nodePath := iter.nodeToPath[nodeIndex]

		if start.Compare(nodePath) <= 0 {
			// [start] is at/before this node and all of its children.
			// All of them should be iterated over.
			iter.nextChildIndex[nodeIndex] = visitNodeNextVal

			if !foundEligibleStartKey || nodePath.Compare(smallestEligibleStartKey) < 0 {
				smallestEligibleStartKey = nodePath
				iter.nextNodeIndex = nodeIndex
				foundEligibleStartKey = true
			}

			continue
		}

		// [start] is after this node's key.
		// Find which children, if any, we should iterate over.
		for childIdx := range node.Children {
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

			if start.Compare(childKey) > 0 {
				// The child is before [start]. Don't iterate over it.
				continue
			}

			// The child is at/after [start].
			if !childIsNode {
				if !foundEligibleStartKey || childKey.Compare(smallestEligibleStartKey) < 0 {
					smallestEligibleStartKey = childKey
					iter.nextNodeIndex = nodeIndex
					foundEligibleStartKey = true
				}
				iter.nextChildIndex[nodeIndex] = int(childIdx)
				break
			}
			// On a subsequent iteration of this loop, we'll find the next child
			// that is at/after [start] and mark that as the next child of [node]
			// to visit because the one at [childIdx] is a node which will be
			// visited first.
		}
		if _, ok := iter.nextChildIndex[nodeIndex]; !ok {
			// We didn't find any children that are at/after [start].
			// Mark this node as exhausted.
			iter.nextChildIndex[nodeIndex] = NodeBranchFactor
		}
	}
	if iter.nextNodeIndex == nextNodeIndexNotFoundVal {
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

	node := i.proof[i.nextNodeIndex]
	childIdx := i.nextChildIndex[i.nextNodeIndex]
	shouldVisitNode := childIdx == visitNodeNextVal

	if shouldVisitNode {
		// The node itself should be visited next.
		i.key = i.nodeToPath[i.nextNodeIndex]
		i.value = i.nodeToID[i.nextNodeIndex]
	} else {
		i.key = i.nodeToPath[i.nextNodeIndex].Append(byte(childIdx))
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
	i.nextChildIndex[i.nextNodeIndex] = nextChildIndex

	// If the next child is in the proof, visit it next.
	if i.nextNodeIndex != len(i.proof)-1 &&
		i.nodeToBranchIndex[i.nextNodeIndex] == byte(nextChildIndex) &&
		len(i.proof[i.nextNodeIndex+1].Children) > 0 {
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
		i.nextChildIndex[i.nextNodeIndex] = newNextChildIndex
		i.nextNodeIndex++
	}

	// If we've visited all the children of this node,
	// ascend to the nearest node that isn't exhausted.
	if nextChildIndex == int(NodeBranchFactor) {
		// Note it's impossible for us to have
		// just descended to the next proof node
		// because there's no branch index at [NodeBranchFactor].
		if i.nextNodeIndex == 0 {
			// We are done with the proof.
			i.exhausted = true
		} else {
			// We are done with this node.
			// Ascend to the node above it, unless we just descended.
			i.nextNodeIndex--
			for i.nextChildIndex[i.nextNodeIndex] == int(NodeBranchFactor) {
				if i.nextNodeIndex == 0 {
					i.exhausted = true
					break
				}
				i.nextNodeIndex--
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
