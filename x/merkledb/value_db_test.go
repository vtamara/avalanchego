// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package merkledb

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/memdb"
)

// Test putting, modifying, deleting, and getting key-node pairs.
func TestValueNodeDB(t *testing.T) {
	require := require.New(t)

	baseDB := memdb.New()

	size := 10
	db := newValueDB(
		baseDB,
		&sync.Pool{
			New: func() interface{} { return make([]byte, 0) },
		},
		&mockMetrics{},
		size,
	)

	// Getting a key that doesn't exist should return an error.
	key := ToKey([]byte{0x01})
	val, err := db.Get(key)
	require.NoError(err)
	require.True(val.IsNothing())

	val1 := []byte{1}
	batch := db.NewBatch()
	batch.Put(key, val1)
	require.NoError(batch.Write())

	// Get the key-node pair.
	val1Read, err := db.Get(key)
	require.NoError(err)
	require.Equal(val1, val1Read.Value())

	// Delete the key-node pair.
	batch = db.NewBatch()
	batch.Delete(key)
	require.NoError(batch.Write())

	// Key should be gone now.
	val, err = db.Get(key)
	require.NoError(err)
	require.True(val.IsNothing())

	// Put a key-node pair and delete it in the same batch.
	batch = db.NewBatch()
	batch.Put(key, val1)
	batch.Delete(key)
	require.NoError(batch.Write())

	// Key should still be gone.
	val, err = db.Get(key)
	require.NoError(err)
	require.True(val.IsNothing())

	// Put a key-node pair and overwrite it in the same batch.
	val2 := []byte{2}

	batch = db.NewBatch()
	batch.Put(key, val1)
	batch.Put(key, val2)
	require.NoError(batch.Write())

	// Get the key-node pair.
	val2Read, err := db.Get(key)
	require.NoError(err)
	require.Equal(val2, val2Read.Value())

	// Overwrite the key-node pair in a subsequent batch.
	batch = db.NewBatch()
	batch.Put(key, val1)
	require.NoError(batch.Write())

	// Get the key-node pair.
	val1Read, err = db.Get(key)
	require.NoError(err)
	require.Equal(val1, val1Read.Value())

	// Get the key-node pair from the database, not the cache.
	db.nodeCache.Flush()
	val1Read, err = db.Get(key)
	require.NoError(err)
	require.Equal(val1, val1Read.Value())

	// Make sure the key is prefixed in the base database.
	it := baseDB.NewIteratorWithPrefix(valuePrefix)
	defer it.Release()
	require.True(it.Next())
	require.False(it.Next())
}

func TestValueNodeDBIterator(t *testing.T) {
	require := require.New(t)

	baseDB := memdb.New()
	cacheSize := 10
	db := newValueDB(
		baseDB,
		&sync.Pool{
			New: func() interface{} { return make([]byte, 0) },
		},
		&mockMetrics{},
		cacheSize,
	)

	// Put key-node pairs.
	for i := 0; i < cacheSize; i++ {
		batch := db.NewBatch()
		batch.Put(ToKey([]byte{byte(i)}), []byte{byte(i)})
		require.NoError(batch.Write())
	}

	// Iterate over the key-node pairs.
	it := db.newIteratorWithStartAndPrefix(nil, nil)

	i := 0
	for it.Next() {
		require.Equal([]byte{byte(i)}, it.Key())
		require.Equal([]byte{byte(i)}, it.Value())
		i++
	}
	require.NoError(it.Error())
	require.Equal(cacheSize, i)
	it.Release()

	// Iterate over the key-node pairs with a start.
	it = db.newIteratorWithStartAndPrefix([]byte{2}, nil)
	i = 0
	for it.Next() {
		require.Equal([]byte{2 + byte(i)}, it.Key())
		require.Equal([]byte{2 + byte(i)}, it.Value())
		i++
	}
	require.NoError(it.Error())
	require.Equal(cacheSize-2, i)
	it.Release()

	// Put key-node pairs with a common prefix.
	key := ToKey([]byte{0xFF, 0x00})
	val := []byte{0xFF, 0x00}
	batch := db.NewBatch()
	batch.Put(key, val)
	require.NoError(batch.Write())

	key = ToKey([]byte{0xFF, 0x01})
	val = []byte{0xFF, 0x01}
	batch = db.NewBatch()
	batch.Put(key, val)
	require.NoError(batch.Write())

	// Iterate over the key-node pairs with a prefix.
	it = db.newIteratorWithStartAndPrefix(nil, []byte{0xFF})
	i = 0
	for it.Next() {
		require.Equal([]byte{0xFF, byte(i)}, it.Key())
		require.Equal([]byte{0xFF, byte(i)}, it.Value())
		i++
	}
	require.NoError(it.Error())
	require.Equal(2, i)

	// Iterate over the key-node pairs with a start and prefix.
	it = db.newIteratorWithStartAndPrefix([]byte{0xFF, 0x01}, []byte{0xFF})
	i = 0
	for it.Next() {
		require.Equal([]byte{0xFF, 0x01}, it.Key())
		require.Equal([]byte{0xFF, 0x01}, it.Value())
		i++
	}
	require.NoError(it.Error())
	require.Equal(1, i)

	// Iterate over closed database.
	it = db.newIteratorWithStartAndPrefix(nil, nil)
	require.True(it.Next())
	require.NoError(it.Error())
	db.Close()
	require.False(it.Next())
	err := it.Error()
	require.ErrorIs(err, database.ErrClosed)
}
