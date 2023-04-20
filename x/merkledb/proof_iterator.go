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
	// Invariant: The node at this index has at least one child
	// which has not been visited yet.
	// TODO make true for root.
	nextNodeIndex int
	// TODO comment.
	nodeToLastVisited map[int]int
	proof             []ProofNode
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
		nodeToLastVisited: map[int]int{},
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
			childKey := nodePath.Append(childIdx)

			if start.Compare(childKey) > 0 {
				// The child is before [start]. Don't iterate over it.
				continue
			}

			// The child is at/after [start].
			// Mark it as the next child to visit at this node.
			iter.nodeToLastVisited[nodeIndex] = int(childIdx) - 1

			if !foundEligibleStartKey || childKey.Compare(smallestEligibleStartKey) < 0 {
				smallestEligibleStartKey = childKey
				iter.nextNodeIndex = nodeIndex
				foundEligibleStartKey = true
			}
			break
		}

		if _, ok := iter.nodeToLastVisited[nodeIndex]; !ok {
			// We didn't find any children that are at/after [start].
			// Mark this node as exhausted.
			iter.nodeToLastVisited[nodeIndex] = NodeBranchFactor
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
	// Note lastVisitedIndex is 0 if we haven't visited this node yet.
	lastVisitedIndex, hasLastVisited := i.nodeToLastVisited[i.nextNodeIndex]
	if !hasLastVisited {
		// Set to -1 so that when we iterate looking for the next child to visit,
		// we start at 0.
		lastVisitedIndex = -1
	}

	// Find the next child index to visit for this node.
	// Note the invariant of [nextNodeIndex] ensures that there is at least one
	// child to visit.
	var childIdx int
	for childIdx = lastVisitedIndex + 1; childIdx < NodeBranchFactor; childIdx++ {
		if _, ok := node.Children[byte(childIdx)]; ok {
			break
		}
	}

	if i.nextNodeIndex == 0 && !hasLastVisited {
		// Special case for returning the root's key.
		// For all other nodes, the parent reports the child's key.
		// However, the root has no parent so we return its key here.
		i.key = i.nodeToPath[i.nextNodeIndex]
		i.value = ids.Empty

		// Set to -1 so that next time [i.nextNodeIndex] is the root,
		// [hasLastVisited] will be true and we won't enter this block.
		i.nodeToLastVisited[i.nextNodeIndex] = -1

		// Set to -1 so that below, when we check whether this node
		// has more children to visit, we start looking for children
		// at index 0.
		childIdx = -1
	} else {
		i.key = i.nodeToPath[i.nextNodeIndex].Append(byte(childIdx))
		i.value = node.Children[byte(childIdx)]
		i.nodeToLastVisited[i.nextNodeIndex] = childIdx
	}

	// Find next key to visit.
	if branchIndex, ok := i.nodeToBranchIndex[i.nextNodeIndex]; ok && branchIndex == byte(childIdx) {
		// We just visited the next node in the proof.
		// Next iteration we should visit the first child
		// of that node, if one exists.
		nextNode := i.proof[i.nextNodeIndex+1]
		if len(nextNode.Children) > 0 {
			i.nextNodeIndex++
			return true
		}
		// The next node has no children to iterate over.
	}

	// Check whether there are more children to visit at this node.
	for nextChildIndex := childIdx + 1; nextChildIndex < NodeBranchFactor; nextChildIndex++ {
		if _, ok := node.Children[byte(nextChildIndex)]; ok {
			// There are more children to visit at this node.
			// [i.nextNodeIndex] is already set to the correct value.
			return true
		}
	}

	// There are no more children to visit at this node.
	// If this is the first node, we're done.
	if i.nextNodeIndex == 0 {
		i.exhausted = true
		return true
	}

	// Go up to the next ancestor node that has more children to visit.
	for i.nextNodeIndex > 0 {
		i.nextNodeIndex--

		// Since we just finished iterating over [node], we know that
		// we're part way through iterating over its parent.
		// Find the index we visited last in the parent.
		parentLastVisited := i.nodeToBranchIndex[i.nextNodeIndex]
		i.nodeToLastVisited[i.nextNodeIndex] = int(parentLastVisited)

		// Check whether there are more children to visit at the parent.
		parent := i.proof[i.nextNodeIndex]
		for nextChildIndex := parentLastVisited + 1; nextChildIndex < NodeBranchFactor; nextChildIndex++ {
			if _, ok := parent.Children[byte(nextChildIndex)]; ok {
				// There are more children to visit at this node.
				// [i.nextNodeIndex] is already set to the correct value.
				return true
			}
		}
	}
	// There are no more nodes with children to visit.
	i.exhausted = true
	return true
}

func (i *proofIterator) Key() path {
	return i.key
}

func (i *proofIterator) Value() ids.ID {
	return i.value
}
