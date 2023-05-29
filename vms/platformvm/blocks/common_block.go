// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package blocks

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/hashing"
)

// CommonBlock contains fields and methods common to all blocks in this VM.
type CommonBlock struct {
	// parent's ID
	PrntID ids.ID `serialize:"true" json:"parentID"`

	// This block's height. The genesis block is at height 0.
	Hght uint64 `serialize:"true" json:"height"`

	BlockID ids.ID `json:"id"`
	bytes   []byte
	version uint16
}

func (b *CommonBlock) initialize(bytes []byte, version uint16) {
	b.BlockID = hashing.ComputeHash256Array(bytes)
	b.bytes = bytes
	b.version = version
}

func (b *CommonBlock) ID() ids.ID {
	return b.BlockID
}

func (b *CommonBlock) Parent() ids.ID {
	return b.PrntID
}

func (b *CommonBlock) Bytes() []byte {
	return b.bytes
}

func (b *CommonBlock) Height() uint64 {
	return b.Hght
}

func (b *CommonBlock) Version() uint16 {
	return b.version
}
