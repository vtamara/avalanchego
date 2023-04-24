// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/vms/components/verify"
)

type ExtendPermissionlessValidatorStakingTx struct {
	// Metadata, inputs and outputs
	BaseTx `serialize:"true"`

	Subnet ids.ID `serialize:"true" json:"subnetID"`

	NodeID ids.NodeID `serialize:"true" json:"nodeID"`

	StakerAuth verify.Verifiable `serialize:"true" json:"stakerAuthorization"`
}

func (tx *ExtendPermissionlessValidatorStakingTx) SyntacticVerify(ctx *snow.Context) error {
	switch {
	case tx == nil:
		return ErrNilTx
	case tx.SyntacticallyVerified: // already passed syntactic verification
		return nil
	case tx.NodeID == ids.EmptyNodeID:
		return errEmptyNodeID
	}

	if err := tx.BaseTx.SyntacticVerify(ctx); err != nil {
		return err
	}
	if err := tx.StakerAuth.Verify(); err != nil {
		return err
	}

	tx.SyntacticallyVerified = true
	return nil
}

func (tx *ExtendPermissionlessValidatorStakingTx) Visit(visitor Visitor) error {
	return visitor.ExtendPermissionlessValidatorStakingTx(tx)
}
