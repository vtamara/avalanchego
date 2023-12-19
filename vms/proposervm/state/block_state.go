// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package state

import (
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/ava-labs/avalanchego/cache"
	"github.com/ava-labs/avalanchego/cache/metercacher"
	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/proposervm/block"
)

const blockCacheSize = 64 * units.MiB

var (
	errBlockWrongVersion = errors.New("wrong version")

	_ BlockState = (*blockState)(nil)
)

type BlockState interface {
	GetInterestingBlockHeights(logging.Logger) (set.Set[ids.ID], error)

	GetBlock(blkID ids.ID) (block.Block, choices.Status, error)
	PutBlock(blk block.Block, status choices.Status) error
	DeleteBlock(blkID ids.ID) error
}

type blockState struct {
	// Caches BlockID -> Block. If the Block is nil, that means the block is not
	// in storage.
	blkCache cache.Cacher[ids.ID, *blockWrapper]

	db database.Database
}

type blockWrapper struct {
	Block  []byte         `serialize:"true"`
	Status choices.Status `serialize:"true"`

	block block.Block
}

func cachedBlockSize(_ ids.ID, bw *blockWrapper) int {
	if bw == nil {
		return ids.IDLen + constants.PointerOverhead
	}
	return ids.IDLen + len(bw.Block) + wrappers.IntLen + 2*constants.PointerOverhead
}

func NewBlockState(db database.Database) BlockState {
	return &blockState{
		blkCache: cache.NewSizedLRU[ids.ID, *blockWrapper](
			blockCacheSize,
			cachedBlockSize,
		),
		db: db,
	}
}

func NewMeteredBlockState(db database.Database, namespace string, metrics prometheus.Registerer) (BlockState, error) {
	blkCache, err := metercacher.New[ids.ID, *blockWrapper](
		fmt.Sprintf("%s_block_cache", namespace),
		metrics,
		cache.NewSizedLRU[ids.ID, *blockWrapper](
			blockCacheSize,
			cachedBlockSize,
		),
	)

	return &blockState{
		blkCache: blkCache,
		db:       db,
	}, err
}

func (s *blockState) GetInterestingBlockHeights(log logging.Logger) (set.Set[ids.ID], error) {
	it := s.db.NewIterator()
	defer it.Release()

	var (
		seenProposers = set.Of(ids.EmptyNodeID)
		blkIDs        set.Set[ids.ID]
	)
	for it.Next() {
		blkWrapper := blockWrapper{}
		parsedVersion, err := c.Unmarshal(it.Value(), &blkWrapper)
		if err != nil {
			return nil, err
		}
		if parsedVersion != version {
			return nil, errBlockWrongVersion
		}

		blkIntf, err := block.Parse(blkWrapper.Block)
		if err != nil {
			return nil, err
		}

		blk, ok := blkIntf.(block.SignedBlock)
		if !ok {
			continue
		}

		nodeID := blk.Proposer()
		if seenProposers.Contains(nodeID) {
			continue
		}
		seenProposers.Add(nodeID)

		blkID := blk.ID()
		log.Info("found new proposer",
			zap.Stringer("nodeID", nodeID),
			zap.Stringer("blkID", blkID),
		)
		blkIDs.Add(blkID)
	}

	return blkIDs, it.Error()
}

func (s *blockState) GetBlock(blkID ids.ID) (block.Block, choices.Status, error) {
	if blk, found := s.blkCache.Get(blkID); found {
		if blk == nil {
			return nil, choices.Unknown, database.ErrNotFound
		}
		return blk.block, blk.Status, nil
	}

	blkWrapperBytes, err := s.db.Get(blkID[:])
	if err == database.ErrNotFound {
		s.blkCache.Put(blkID, nil)
		return nil, choices.Unknown, database.ErrNotFound
	}
	if err != nil {
		return nil, choices.Unknown, err
	}

	blkWrapper := blockWrapper{}
	parsedVersion, err := c.Unmarshal(blkWrapperBytes, &blkWrapper)
	if err != nil {
		return nil, choices.Unknown, err
	}
	if parsedVersion != version {
		return nil, choices.Unknown, errBlockWrongVersion
	}

	// The key was in the database
	blk, err := block.Parse(blkWrapper.Block)
	if err != nil {
		return nil, choices.Unknown, err
	}
	blkWrapper.block = blk

	s.blkCache.Put(blkID, &blkWrapper)
	return blk, blkWrapper.Status, nil
}

func (s *blockState) PutBlock(blk block.Block, status choices.Status) error {
	blkWrapper := blockWrapper{
		Block:  blk.Bytes(),
		Status: status,
		block:  blk,
	}

	bytes, err := c.Marshal(version, &blkWrapper)
	if err != nil {
		return err
	}

	blkID := blk.ID()
	s.blkCache.Put(blkID, &blkWrapper)
	return s.db.Put(blkID[:], bytes)
}

func (s *blockState) DeleteBlock(blkID ids.ID) error {
	s.blkCache.Evict(blkID)
	return s.db.Delete(blkID[:])
}
