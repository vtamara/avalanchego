// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package merkledb

import (
	"errors"
	"github.com/ava-labs/avalanchego/utils/maybe"
	"sync"

	"github.com/ava-labs/avalanchego/cache"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/utils"
)

var _ database.Iterator = (*iterator)(nil)

type valueDB struct {
	// Holds unused []byte
	bufferPool *sync.Pool

	// The underlying storage.
	// Keys written to [baseDB] are prefixed with [valueNodePrefix].
	baseDB database.Database

	// If a value is nil, the corresponding key isn't in the trie.
	// Paths in [valueCache] aren't prefixed with [valueNodePrefix].
	valueCache cache.Cacher[Key, maybe.Maybe[[]byte]]
	metrics    merkleMetrics

	closed utils.Atomic[bool]
}

func newValueNodeDB(
	db database.Database,
	bufferPool *sync.Pool,
	metrics merkleMetrics,
	cacheSize int,
) *valueDB {
	return &valueDB{
		metrics:    metrics,
		baseDB:     db,
		bufferPool: bufferPool,
		valueCache: cache.NewSizedLRU(cacheSize, func(k Key, v maybe.Maybe[[]byte]) int {
			if v.IsNothing() {
				return len(k.value)
			}
			return len(k.value) + len(v.Value())
		}),
	}
}

func (db *valueDB) newIteratorWithStartAndPrefix(start, prefix []byte) database.Iterator {
	prefixedStart := addPrefixToKey(db.bufferPool, valueNodePrefix, start)
	prefixedPrefix := addPrefixToKey(db.bufferPool, valueNodePrefix, prefix)
	i := &iterator{
		db:        db,
		valueIter: db.baseDB.NewIteratorWithStartAndPrefix(prefixedStart, prefixedPrefix),
	}
	db.bufferPool.Put(prefixedStart)
	db.bufferPool.Put(prefixedPrefix)
	return i
}

func (db *valueDB) Close() {
	db.closed.Set(true)
}

func (db *valueDB) NewBatch() *valueNodeBatch {
	return &valueNodeBatch{
		db:  db,
		ops: make(map[Key]maybe.Maybe[[]byte], defaultBufferLength),
	}
}

func (db *valueDB) Get(key Key) (maybe.Maybe[[]byte], error) {
	if cachedValue, isCached := db.valueCache.Get(key); isCached {
		db.metrics.ValueCacheHit()
		return cachedValue, nil
	}
	db.metrics.ValueCacheMiss()

	prefixedKey := addPrefixToKey(db.bufferPool, valueNodePrefix, key.Bytes())
	defer db.bufferPool.Put(prefixedKey)

	db.metrics.DatabaseNodeRead()
	val, err := db.baseDB.Get(prefixedKey)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return maybe.Nothing[[]byte](), nil
		}
		return maybe.Nothing[[]byte](), err
	}
	return maybe.Some(val), nil
}

func (db *valueDB) Clear() error {
	db.valueCache.Flush()
	return database.AtomicClearPrefix(db.baseDB, db.baseDB, valueNodePrefix)
}

// Batch of database operations
type valueNodeBatch struct {
	db  *valueDB
	ops map[Key]maybe.Maybe[[]byte]
}

func (b *valueNodeBatch) Put(key Key, value maybe.Maybe[[]byte]) {
	b.ops[key] = value
}

func (b *valueNodeBatch) Delete(key Key) {
	b.ops[key] = maybe.Nothing[[]byte]()
}

// Write flushes any accumulated data to the underlying database.
func (b *valueNodeBatch) Write() error {
	dbBatch := b.db.baseDB.NewBatch()
	for key, value := range b.ops {
		b.db.metrics.DatabaseNodeWrite()
		b.db.valueCache.Put(key, value)
		prefixedKey := addPrefixToKey(b.db.bufferPool, valueNodePrefix, key.Bytes())
		if value.IsNothing() {
			if err := dbBatch.Delete(prefixedKey); err != nil {
				return err
			}
		} else if err := dbBatch.Put(prefixedKey, value.Value()); err != nil {
			return err
		}

		b.db.bufferPool.Put(prefixedKey)
	}

	return dbBatch.Write()
}

type iterator struct {
	db        *valueDB
	valueIter database.Iterator
	err       error
}

func (i *iterator) Error() error {
	if i.err != nil {
		return i.err
	}
	if i.db.closed.Get() {
		return database.ErrClosed
	}
	return i.valueIter.Error()
}

func (i *iterator) Key() []byte {
	key := i.valueIter.Key()
	if len(key) > valueNodePrefixLen {
		return key[valueNodePrefixLen:]
	}
	return key
}

func (i *iterator) Value() []byte {
	return i.valueIter.Value()
}

func (i *iterator) Next() bool {
	if i.Error() != nil || i.db.closed.Get() {
		return false
	}

	i.db.metrics.DatabaseNodeRead()
	return i.valueIter.Next()
}

func (i *iterator) Release() {
	i.valueIter.Release()
}
