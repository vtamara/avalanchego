// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package merkledb

import (
	"sync"

	"github.com/ava-labs/avalanchego/database"
)

const defaultBufferLength = 256

// Holds intermediate nodes. That is, those without values.
// Changes to this database aren't written to [baseDB] until
// they're evicted from the [nodeCache] or Flush is called.
type childrenDB struct {
	// Holds unused []byte
	bufferPool *sync.Pool

	// The underlying storage.
	// Keys written to [baseDB] are prefixed with [childrenPrefix].
	baseDB database.Database

	// If a value is nil, the corresponding key isn't in the trie.
	// Note that a call to Put may cause a node to be evicted
	// from the cache, which will call [OnEviction].
	// A non-nil error returned from Put is considered fatal.
	// Keys in [nodeCache] aren't prefixed with [childrenPrefix].
	nodeCache onEvictCache[Key, nodeChildren]
	// the number of bytes to evict during an eviction batch
	evictionBatchSize int
	metrics           merkleMetrics
	tokenSize         int
}

func newChildrenDB(
	db database.Database,
	bufferPool *sync.Pool,
	metrics merkleMetrics,
	size int,
	evictionBatchSize int,
	tokenSize int,
) *childrenDB {
	result := &childrenDB{
		metrics:           metrics,
		baseDB:            db,
		bufferPool:        bufferPool,
		evictionBatchSize: evictionBatchSize,
		tokenSize:         tokenSize,
	}
	result.nodeCache = newOnEvictCache(
		size,
		func(key Key, children nodeChildren) int {
			total := len(key.value) + 8
			for _, child := range children {
				total += len(child.id) + len(child.compressedKey.value) + 10
			}
			return total
		},
		result.onEviction,
	)
	return result
}

// A non-nil error is considered fatal and closes [db.baseDB].
func (db *childrenDB) onEviction(key Key, children nodeChildren) error {
	writeBatch := db.baseDB.NewBatch()

	totalSize := db.nodeCache.size(key, children)
	if err := db.addToBatch(writeBatch, key, children); err != nil {
		_ = db.baseDB.Close()
		return err
	}

	// Evict the oldest [evictionBatchSize] nodes from the cache
	// and write them to disk. We write a batch of them, rather than
	// just [n], so that we don't immediately evict and write another
	// node, because each time this method is called we do a disk write.
	// Evicts a total number of bytes, rather than a number of nodes
	for totalSize < db.evictionBatchSize {
		key, n, exists := db.nodeCache.removeOldest()
		if !exists {
			// The cache is empty.
			break
		}
		totalSize += db.nodeCache.size(key, n)
		if err := db.addToBatch(writeBatch, key, n); err != nil {
			_ = db.baseDB.Close()
			return err
		}
	}
	if err := writeBatch.Write(); err != nil {
		_ = db.baseDB.Close()
		return err
	}
	return nil
}

func (db *childrenDB) addToBatch(b database.Batch, key Key, children map[byte]*child) error {
	dbKey := db.constructDBKey(key)
	defer db.bufferPool.Put(dbKey)
	db.metrics.DatabaseNodeWrite()
	if children == nil {
		return b.Delete(dbKey)
	}
	return b.Put(dbKey, codec.encodeChildren(children))
}

func (db *childrenDB) Get(key Key) (map[byte]*child, error) {
	if cachedValue, isCached := db.nodeCache.Get(key); isCached {
		db.metrics.IntermediateNodeCacheHit()
		if cachedValue == nil {
			return nil, database.ErrNotFound
		}
		return cachedValue, nil
	}
	db.metrics.IntermediateNodeCacheMiss()

	dbKey := db.constructDBKey(key)
	db.metrics.DatabaseNodeRead()
	nodeBytes, err := db.baseDB.Get(dbKey)
	if err != nil {
		return nil, err
	}
	db.bufferPool.Put(dbKey)

	return codec.decodeChildren(nodeBytes)
}

// constructDBKey returns a key that can be used in [db.baseDB].
// We need to be able to differentiate between two keys of equal
// byte length but different token length, so we add padding to differentiate.
// Additionally, we add a prefix indicating it is part of the childrenDB.
func (db *childrenDB) constructDBKey(key Key) []byte {
	if db.tokenSize == 8 {
		// For tokens of size byte, no padding is needed since byte length == token length
		return addPrefixToKey(db.bufferPool, childrenPrefix, key.Bytes())
	}

	return addPrefixToKey(db.bufferPool, childrenPrefix, key.Extend(ToToken(1, db.tokenSize)).Bytes())
}

func (db *childrenDB) Put(key Key, children map[byte]*child) error {
	return db.nodeCache.Put(key, children)
}

func (db *childrenDB) Flush() error {
	return db.nodeCache.Flush()
}

func (db *childrenDB) Delete(key Key) error {
	return db.nodeCache.Put(key, nil)
}
