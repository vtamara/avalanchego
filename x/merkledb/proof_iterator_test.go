// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package merkledb

import (
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/stretchr/testify/require"
)

// TODO expand this test to include other aspects of the function.
func TestNewProofIterator(t *testing.T) {
	type test struct {
		name                 string
		proof                []ProofNode
		start                path
		expectedNodeIndex    int
		expectedExhausted    bool
		expectedChildIndices map[int]int
	}

	tests := []test{
		{
			name: "one node; iterate over node and all children",
			proof: []ProofNode{
				{
					KeyPath: SerializedPath{
						Value: []byte{0},
					},
				},
			},
			start:                "",
			expectedNodeIndex:    0,
			expectedExhausted:    false,
			expectedChildIndices: map[int]int{},
		},
		{
			name: "one node; iterate over all children but not node",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
					Children: map[byte]ids.ID{
						0: ids.GenerateTestID(),
					},
				},
			},
			start:                path([]byte{0}),
			expectedNodeIndex:    0,
			expectedExhausted:    false,
			expectedChildIndices: map[int]int{},
		},
		{
			name: "one node; no children; exhausted",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
				},
			},
			start:                path([]byte{0, 1}),
			expectedNodeIndex:    0,
			expectedExhausted:    true,
			expectedChildIndices: map[int]int{0: 16},
		},
		{
			name: "one node; children; exhausted",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
					Children: map[byte]ids.ID{
						0: ids.GenerateTestID(),
					},
				},
			},
			start:                path([]byte{0, 1}),
			expectedNodeIndex:    0,
			expectedExhausted:    true,
			expectedChildIndices: map[int]int{0: 16},
		},
		{
			name: "one node; start at first child",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
					Children: map[byte]ids.ID{
						0: ids.GenerateTestID(),
					},
				},
			},
			start:                path([]byte{0, 0}),
			expectedNodeIndex:    0,
			expectedExhausted:    false,
			expectedChildIndices: map[int]int{0: 0},
		},
		{
			name: "one node; start at second child",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
					Children: map[byte]ids.ID{
						0: ids.GenerateTestID(),
						1: ids.GenerateTestID(),
					},
				},
			},
			start:                path([]byte{0, 1}),
			expectedNodeIndex:    0,
			expectedExhausted:    false,
			expectedChildIndices: map[int]int{0: 1},
		},
		{
			name: "two nodes; first with children; start at second node",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
					Children: map[byte]ids.ID{
						0: ids.GenerateTestID(),
					},
				},
				{
					KeyPath: path([]byte{0, 0}).Serialize(),
					Children: map[byte]ids.ID{
						1: ids.GenerateTestID(),
					},
				},
			},
			start:                path([]byte{0, 0}),
			expectedNodeIndex:    0,
			expectedExhausted:    false,
			expectedChildIndices: map[int]int{0: 0},
		},
		{
			name: "two nodes; start at second node's first child",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
					Children: map[byte]ids.ID{
						0: ids.GenerateTestID(),
					},
				},
				{
					KeyPath: path([]byte{0, 0}).Serialize(),
					Children: map[byte]ids.ID{
						1: ids.GenerateTestID(),
					},
				},
			},
			start:             path([]byte{0, 0, 1}),
			expectedNodeIndex: 1,
			expectedExhausted: false,
			expectedChildIndices: map[int]int{
				0: 16,
				1: 1,
			},
		},
		{
			name: "two nodes; start at second node's second child",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
					Children: map[byte]ids.ID{
						0: ids.GenerateTestID(),
					},
				},
				{
					KeyPath: path([]byte{0, 0}).Serialize(),
					Children: map[byte]ids.ID{
						1: ids.GenerateTestID(),
						2: ids.GenerateTestID(),
					},
				},
			},
			start:             path([]byte{0, 0, 2}),
			expectedNodeIndex: 1,
			expectedExhausted: false,
			expectedChildIndices: map[int]int{
				0: 16,
				1: 2,
			},
		},
		{
			name: "two nodes; exhausted",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
					Children: map[byte]ids.ID{
						0: ids.GenerateTestID(),
					},
				},
				{
					KeyPath: path([]byte{0, 0}).Serialize(),
					Children: map[byte]ids.ID{
						1: ids.GenerateTestID(),
						2: ids.GenerateTestID(),
					},
				},
			},
			start:             path([]byte{0, 0, 3}),
			expectedNodeIndex: 1,
			expectedExhausted: true,
			expectedChildIndices: map[int]int{
				0: 16,
				1: 16,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			// TODO test first key/value is right
			iter := newProofIterator(tt.proof, tt.start)
			require.Equal(tt.expectedNodeIndex, iter.nodeIndex)
			require.Equal(tt.expectedExhausted, iter.exhausted)
			require.Equal(tt.expectedChildIndices, iter.nextChildIndex)
		})
	}
}
