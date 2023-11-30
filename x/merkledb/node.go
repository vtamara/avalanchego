// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package merkledb

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/utils/maybe"
	"golang.org/x/exp/slices"
)

const HashLength = 32

// Representation of a node stored in the database.
type dbNode struct {
	hasValue bool
	children map[byte]*child
}

type child struct {
	compressedKey Key
	id            ids.ID
}

// node holds additional information on top of the dbNode that makes calculations easier to do
type node struct {
	dbNode
	key Key
}

// Returns a new node with the given [key] and no value.
func newNode(key Key) *node {
	return &node{
		dbNode: dbNode{
			children: make(map[byte]*child, 2),
		},
		key: key,
	}
}

// Parse [nodeBytes] to a node and set its key to [key].
func parseNode(key Key, nodeBytes []byte) (*node, error) {
	n := dbNode{}
	if err := codec.decodeDBNode(nodeBytes, &n); err != nil {
		return nil, err
	}
	result := &node{
		dbNode: n,
		key:    key,
	}
	return result, nil
}

// Returns the byte representation of this node.
func (n *node) bytes() []byte {
	return codec.encodeDBNode(&n.dbNode)
}

// Returns and caches the ID of this node.
func (n *node) calculateID(metrics merkleMetrics, value maybe.Maybe[[]byte]) ids.ID {
	metrics.HashCalculated()
	bytes := codec.encodeHashValues(n, value)
	return hashing.ComputeHash256Array(bytes)
}

// Adds [child] as a child of [n].
// Assumes [child]'s key is valid as a child of [n].
// That is, [n.key] is a prefix of [child.key].
func (n *node) addChild(childNode *node, tokenSize int) {
	n.setChildEntry(
		childNode.key.Token(n.key.length, tokenSize),
		&child{
			compressedKey: childNode.key.Skip(n.key.length + tokenSize),
		},
	)
}

// Adds a child to [n] without a reference to the child node.
func (n *node) setChildEntry(index byte, childEntry *child) {
	n.children[index] = childEntry
}

// Removes [child] from [n]'s children.
func (n *node) removeChild(child *node, tokenSize int) {
	delete(n.children, child.key.Token(n.key.length, tokenSize))
}

// clone Returns a copy of [n].
// Note: value isn't cloned because it is never edited, only overwritten
// if this ever changes, value will need to be copied as well
// it is safe to clone all fields because they are only written/read while one or both of the db locks are held
func (n *node) clone() *node {
	result := &node{
		key: n.key,
		dbNode: dbNode{
			hasValue: n.hasValue,
			children: make(map[byte]*child, len(n.children)),
		},
	}
	for key, existing := range n.children {
		result.children[key] = &child{
			compressedKey: existing.compressedKey,
			id:            existing.id,
		}
	}
	return result
}

func getValueOrDigest(value maybe.Maybe[[]byte], clone bool) maybe.Maybe[[]byte] {
	if value.IsNothing() || len(value.Value()) <= HashLength {
		if clone {
			return maybe.Bind(value, slices.Clone[[]byte])
		}
		return value
	} else {
		return maybe.Some(hashing.ComputeHash256(value.Value()))
	}
}

// Returns the ProofNode representation of this node.
func (n *node) asProofNode(value maybe.Maybe[[]byte]) ProofNode {
	pn := ProofNode{
		Key:         n.key,
		Children:    make(map[byte]ids.ID, len(n.children)),
		ValueOrHash: getValueOrDigest(value, true),
	}
	for index, entry := range n.children {
		pn.Children[index] = entry.id
	}
	return pn
}
