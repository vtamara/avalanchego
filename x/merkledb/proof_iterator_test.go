// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package merkledb

import (
	"bytes"
	"context"
	"math/rand"
	"testing"

	"golang.org/x/exp/slices"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/ids"
)

type keyValue struct {
	key   path
	value ids.ID
}

func TestProofIterator(t *testing.T) {
	testIDs := []ids.ID{}
	for i := 0; i < 10; i++ {
		testIDs = append(testIDs, ids.GenerateTestID())
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
					key:   path([]byte{0, 0}),
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
					key:   path([]byte{0, 0}),
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
					key:   path([]byte{0, 0}),
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
					key:   path([]byte{0, 0}),
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
					key:   path([]byte{0, 0}),
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
			iter := NewProofIterator(tt.proof, tt.start)

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

func TestProofIteratorRandom(t *testing.T) {
	rand := rand.New(rand.NewSource(1337))
	require := require.New(t)

	db, err := getBasicDB()
	require.NoError(err)

	var (
		numProofsToTest  = 1_000
		numKeyValues     = 1_000
		maxKeyLen        = 256
		maxValLen        = 256
		maxRangeStartLen = 8
		maxRangeEndLen   = 8
		maxIterStartLen  = 8
		maxProofLen      = 128
	)

	// Put random keys into the database
	for i := 0; i < numKeyValues; i++ {
		key := make([]byte, rand.Intn(maxKeyLen))
		_, _ = rand.Read(key)
		val := make([]byte, rand.Intn(maxValLen))
		err := db.Put(key, val)
		require.NoError(err)
	}

	for proofIndex := 0; proofIndex < numProofsToTest; proofIndex++ {
		// Generate a proof for a random key
		var (
			rangeStart []byte
			rangeEnd   []byte
		)
		for rangeStart == nil || bytes.Compare(rangeStart, rangeEnd) == 1 {
			rangeStart = make([]byte, rand.Intn(maxRangeStartLen)+1)
			_, _ = rand.Read(rangeStart)
			rangeEnd = make([]byte, rand.Intn(maxRangeEndLen)+1)
			_, _ = rand.Read(rangeEnd)
		}

		proof, err := db.GetRangeProof(
			context.Background(),
			rangeStart,
			rangeEnd,
			rand.Intn(maxProofLen)+1,
		)
		require.NoError(err)

		if len(proof.StartProof) == 0 {
			// Skip since there's no proof to use.
			// This will probbably never happen.
			continue
		}

		// Generate iteration start
		iterStartBytes := make([]byte, rand.Intn(maxIterStartLen)+1)
		_, _ = rand.Read(iterStartBytes)
		iterStartPath := path(iterStartBytes)

		// Generate expected result
		expectedKeyValues := []keyValue{}
		for i, node := range proof.StartProof {
			if i == 0 {
				// Special case the root
				expectedKeyValues = append(expectedKeyValues, keyValue{
					key:   node.KeyPath.deserialize(),
					value: ids.Empty,
				})
			}
			for childIdx, childID := range node.Children {
				expectedKeyValues = append(expectedKeyValues, keyValue{
					key:   node.KeyPath.deserialize().Append(childIdx),
					value: childID,
				})
			}
		}

		// Sort expected key values to (hopefully) match iterator
		slices.SortStableFunc(expectedKeyValues, func(kv1, kv2 keyValue) bool {
			return kv1.key.Compare(kv2.key) < 0
		})

		// Remove all keys that are less than startPath
		firstValidIdx := len(expectedKeyValues)
		for j, kv := range expectedKeyValues {
			if kv.key.Compare(iterStartPath) >= 0 {
				firstValidIdx = j
				break
			}
		}
		expectedKeyValues = expectedKeyValues[firstValidIdx:]

		iter := NewProofIterator(proof.StartProof, iterStartPath)
		numIters := 0
		for iter.Next() {
			require.Less(numIters, len(expectedKeyValues), "too many iterations")

			gotKey := iter.Key()
			require.Equal(expectedKeyValues[numIters].key, gotKey, "failed on proof %v iteration %v", proofIndex, numIters)

			gotValue := iter.Value()
			require.Equal(expectedKeyValues[numIters].value, gotValue, "failed on proof %v iteration %v", proofIndex, numIters)
			numIters++
		}
		require.Equal(len(expectedKeyValues), numIters, "failed on proof %v", proofIndex)
	}

}
