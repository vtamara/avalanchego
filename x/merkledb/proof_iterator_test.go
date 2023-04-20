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

func TestProofIterator(t *testing.T) {
	testIDs := []ids.ID{}
	for i := 0; i < 10; i++ {
		testIDs = append(testIDs, ids.GenerateTestID())
	}

	type keyValue struct {
		key   path
		value ids.ID
	}

	type test struct {
		name              string
		proof             []ProofNode
		start             path
		expectedKeyValues []keyValue
	}

	tests := []test{
		{
			name: "1 node with no children; start after node",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
				},
			},
			start:             path([]byte{0, 1}),
			expectedKeyValues: []keyValue{},
		},
		{
			name: "1 node with no children; start at node",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
				},
			},
			start: path([]byte{0}),
			expectedKeyValues: []keyValue{
				{
					key:   path([]byte{0}),
					value: ids.Empty,
				},
			},
		},
		{
			name: "1 node with no children; start before node",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{1}).Serialize(),
				},
			},
			start: path([]byte{0}),
			expectedKeyValues: []keyValue{
				{
					key:   path([]byte{1}),
					value: ids.Empty,
				},
			},
		},
		{
			name: "1 node with 1 non-node child; iterate over all",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
					Children: map[byte]ids.ID{
						0: testIDs[0],
					},
				},
			},
			start: path([]byte{0}),
			expectedKeyValues: []keyValue{
				{
					key:   path([]byte{0}),
					value: ids.Empty,
				},
				{
					key:   path([]byte{0, 0}),
					value: testIDs[0],
				},
			},
		},
		{
			name: "1 node with 1 non-node child; iterate over only child",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
					Children: map[byte]ids.ID{
						0: testIDs[0],
					},
				},
			},
			start: path([]byte{0, 0}),
			expectedKeyValues: []keyValue{
				{
					key:   path([]byte{0, 0}),
					value: testIDs[0],
				},
			},
		},
		{
			name: "1 node with 1 non-node child; iterate over none",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
					Children: map[byte]ids.ID{
						0: testIDs[0],
					},
				},
			},
			start:             path([]byte{0, 0, 1}), // Greater than any key in proof
			expectedKeyValues: []keyValue{},
		},
		{
			name: "1 node with multiple non-node children; iterate over none",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
					Children: map[byte]ids.ID{
						0: testIDs[0],
						1: testIDs[1],
					},
				},
			},
			start:             path([]byte{1}), // Greater than any key in proof
			expectedKeyValues: []keyValue{},
		},
		{
			name: "1 node with multiple non-node children; iterate over last child",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
					Children: map[byte]ids.ID{
						0: testIDs[0],
						1: testIDs[1],
					},
				},
			},
			start: path([]byte{0, 1}),
			expectedKeyValues: []keyValue{
				{
					key:   path([]byte{0, 1}),
					value: testIDs[1],
				},
			},
		},
		{
			name: "1 node with multiple non-node children; iterate over all",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
					Children: map[byte]ids.ID{
						0: testIDs[0],
						1: testIDs[1],
					},
				},
			},
			start: path([]byte{0}),
			expectedKeyValues: []keyValue{
				{
					key:   path([]byte{0}),
					value: ids.Empty,
				},
				{
					key:   path([]byte{0, 0}),
					value: testIDs[0],
				},
				{
					key:   path([]byte{0, 1}),
					value: testIDs[1],
				},
			},
		},
		{
			name: "1 node with 1 node child; iterate over none",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
					Children: map[byte]ids.ID{
						0: testIDs[0],
					},
				},
				{
					KeyPath:  path([]byte{0, 0, 1}).Serialize(),
					Children: map[byte]ids.ID{},
				},
			},
			start:             path([]byte{1}), // Greater than any key in proof
			expectedKeyValues: []keyValue{},
		},
		{
			name: "1 node with 1 node child; iterate over only child",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
					Children: map[byte]ids.ID{
						0: testIDs[0],
					},
				},
				{
					KeyPath:  path([]byte{0, 0, 1}).Serialize(),
					Children: map[byte]ids.ID{},
				},
			},
			start: path([]byte{0, 0}),
			expectedKeyValues: []keyValue{
				{
					key:   path([]byte{0, 0, 1}),
					value: testIDs[0],
				},
			},
		},
		{
			name: "1 node with 1 node child and non-node children; iterate over all",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
					Children: map[byte]ids.ID{
						0: testIDs[0],
						1: testIDs[1],
					},
				},
				{
					KeyPath: path([]byte{0, 0, 1}).Serialize(),
					Children: map[byte]ids.ID{
						1: testIDs[2],
					},
				},
			},
			start: path([]byte{0}),
			expectedKeyValues: []keyValue{
				{
					key:   path([]byte{0}),
					value: ids.Empty,
				},
				{
					key:   path([]byte{0, 0, 1}),
					value: testIDs[0],
				},
				{
					key:   path([]byte{0, 0, 1, 1}),
					value: testIDs[2],
				},
				{
					key:   path([]byte{0, 1}),
					value: testIDs[1],
				},
			},
		},
		{
			name: "1 node with 1 node child and non-node children; skip root",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
					Children: map[byte]ids.ID{
						0: testIDs[0],
						1: testIDs[1],
					},
				},
				{
					KeyPath: path([]byte{0, 0, 1}).Serialize(),
					Children: map[byte]ids.ID{
						1: testIDs[2],
					},
				},
			},
			start: path([]byte{0, 0}),
			expectedKeyValues: []keyValue{
				{
					key:   path([]byte{0, 0, 1}),
					value: testIDs[0],
				},
				{
					key:   path([]byte{0, 0, 1, 1}),
					value: testIDs[2],
				},
				{
					key:   path([]byte{0, 1}),
					value: testIDs[1],
				},
			},
		},
		{
			name: "3 proof nodes; iterate over all",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
					Children: map[byte]ids.ID{
						0: testIDs[0],
						2: testIDs[1],
					},
				},
				{
					KeyPath: path([]byte{0, 0, 1}).Serialize(),
					Children: map[byte]ids.ID{
						0: testIDs[4],
						2: testIDs[2],
					},
				},
				{
					KeyPath: path([]byte{0, 0, 1, 2}).Serialize(),
					Children: map[byte]ids.ID{
						NodeBranchFactor - 1: testIDs[3],
					},
				},
			},
			start: path([]byte{0}),
			expectedKeyValues: []keyValue{
				{
					key:   path([]byte{0}),
					value: ids.Empty,
				},
				{
					key:   path([]byte{0, 0, 1}),
					value: testIDs[0],
				},
				{
					key:   path([]byte{0, 0, 1, 0}),
					value: testIDs[4],
				},
				{
					key:   path([]byte{0, 0, 1, 2}),
					value: testIDs[2],
				},
				{
					key:   path([]byte{0, 0, 1, 2, NodeBranchFactor - 1}),
					value: testIDs[3],
				},
				{
					key:   path([]byte{0, 2}),
					value: testIDs[1],
				},
			},
		},
		{
			name: "3 proof nodes; skip first",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
					Children: map[byte]ids.ID{
						0: testIDs[0],
						2: testIDs[1],
					},
				},
				{
					KeyPath: path([]byte{0, 0, 1}).Serialize(),
					Children: map[byte]ids.ID{
						0: testIDs[4],
						2: testIDs[2],
					},
				},
				{
					KeyPath: path([]byte{0, 0, 1, 2}).Serialize(),
					Children: map[byte]ids.ID{
						NodeBranchFactor - 1: testIDs[3],
					},
				},
			},
			start: path([]byte{0, 0}),
			expectedKeyValues: []keyValue{
				{
					key:   path([]byte{0, 0, 1}),
					value: testIDs[0],
				},
				{
					key:   path([]byte{0, 0, 1, 0}),
					value: testIDs[4],
				},
				{
					key:   path([]byte{0, 0, 1, 2}),
					value: testIDs[2],
				},
				{
					key:   path([]byte{0, 0, 1, 2, NodeBranchFactor - 1}),
					value: testIDs[3],
				},
				{
					key:   path([]byte{0, 2}),
					value: testIDs[1],
				},
			},
		},
		{
			name: "3 proof nodes; skip first two",
			proof: []ProofNode{
				{
					KeyPath: path([]byte{0}).Serialize(),
					Children: map[byte]ids.ID{
						1: testIDs[0],
						2: testIDs[1],
					},
				},
				{
					KeyPath: path([]byte{0, 1, 1}).Serialize(),
					Children: map[byte]ids.ID{
						0: testIDs[4],
						2: testIDs[2],
					},
				},
				{
					KeyPath: path([]byte{0, 1, 1, 2}).Serialize(),
					Children: map[byte]ids.ID{
						NodeBranchFactor - 1: testIDs[3],
					},
				},
			},
			start: path([]byte{0, 1, 1, 2}),
			expectedKeyValues: []keyValue{
				{
					key:   path([]byte{0, 1, 1, 2}),
					value: testIDs[2],
				},
				{
					key:   path([]byte{0, 1, 1, 2, NodeBranchFactor - 1}),
					value: testIDs[3],
				},
				{
					key:   path([]byte{0, 2}),
					value: testIDs[1],
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)

			// Sanity check test parameters.
			iter := newProofIterator(tt.proof, tt.start)

			i := 0
			for iter.Next() {
				require.Less(i, len(tt.expectedKeyValues), "too many iterations")
				require.Equal(tt.expectedKeyValues[i].key, iter.Key())
				require.Equal(tt.expectedKeyValues[i].value, iter.Value())
				i++
			}
			require.Equal(len(tt.expectedKeyValues), i)
		})
	}
}
