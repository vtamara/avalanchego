// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package merkledb

import (
	"context"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/maybe"
)

type MerkleRootGetter interface {
	// GetMerkleRoot returns the merkle root of the Trie
	GetMerkleRoot(ctx context.Context) (ids.ID, error)
}

type ProofGetter interface {
	// GetProof generates a proof of the value associated with a particular key,
	// or a proof of its absence from the trie
	GetProof(ctx context.Context, bytesPath []byte) (*Proof, error)
}

type ReadOnlyTrie interface {
	MerkleRootGetter
	ProofGetter

	// GetValue gets the value associated with the specified key
	// database.ErrNotFound if the key is not present
	GetValue(ctx context.Context, key []byte) ([]byte, error)

	// GetValues gets the values associated with the specified keys
	// database.ErrNotFound if the key is not present
	GetValues(ctx context.Context, keys [][]byte) ([][]byte, []error)

	// get the value associated with the key in path form
	// database.ErrNotFound if the key is not present
	getValue(key Path) ([]byte, error)

	getNode(key Path, hasValue bool) (*node, error)
	getRoot() *node

	// GetRangeProof returns a proof of up to [maxLength] key-value pairs with
	// keys in range [start, end].
	// If [start] is Nothing, there's no lower bound on the range.
	// If [end] is Nothing, there's no upper bound on the range.
	GetRangeProof(ctx context.Context, start maybe.Maybe[[]byte], end maybe.Maybe[[]byte], maxLength int) (*RangeProof, error)

	database.Iteratee
}

type ViewChanges struct {
	BatchOps []database.BatchOp
	MapOps   map[string]maybe.Maybe[[]byte]
	// ConsumeBytes when set to true will skip copying of bytes and assume
	// ownership of the provided bytes.
	ConsumeBytes bool
}

type Trie interface {
	ReadOnlyTrie

	// NewView returns a new view on top of this Trie where the passed changes
	// have been applied.
	NewView(
		ctx context.Context,
		changes ViewChanges,
	) (TrieView, error)
}

type TrieView interface {
	Trie

	// CommitToDB writes the changes in this view to the database.
	// Takes the DB commit lock.
	CommitToDB(ctx context.Context) error
}

// Returns the nodes along the path to [key].
// The first node is the root, and the last node is either the node with the
// given [key], if it's in the trie, or the node with the largest prefix of
// the [key] if it isn't in the trie.
// Always returns at least the root node.
func getPathTo(t ReadOnlyTrie, key Path) ([]*node, error) {
	var (
		// all node paths start at the root
		currentNode      = t.getRoot()
		matchedPathIndex = 0
		nodes            = []*node{currentNode}
	)

	// while the entire path hasn't been matched
	for matchedPathIndex < key.tokensLength {
		// confirm that a child exists and grab its ID before attempting to load it
		nextChildEntry, hasChild := currentNode.children[key.Token(matchedPathIndex)]

		// the current token for the child entry has now been handled, so increment the matchedPathIndex
		matchedPathIndex += 1

		if !hasChild || !key.Skip(matchedPathIndex).HasPrefix(nextChildEntry.compressedPath) {
			// there was no child along the path or the child that was there doesn't match the remaining path
			return nodes, nil
		}

		// the compressed path of the entry there matched the path, so increment the matched index
		matchedPathIndex += nextChildEntry.compressedPath.tokensLength

		// grab the next node along the path
		var err error
		currentNode, err = t.getNode(key.Take(matchedPathIndex), nextChildEntry.hasValue)
		if err != nil {
			return nil, err
		}

		// add node to path
		nodes = append(nodes, currentNode)
	}
	return nodes, nil
}
