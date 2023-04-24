// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package txs

import (
	"testing"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func TestExtendPermissionlessValidatorStakingTxSyntacticVerify(t *testing.T) {
	type test struct {
		name        string
		txFunc      func(*gomock.Controller) *ExtendPermissionlessValidatorStakingTx
		expectedErr error
	}

	var (
		networkID = uint32(1337)
		chainID   = ids.GenerateTestID()
	)

	ctx := &snow.Context{
		ChainID:   chainID,
		NetworkID: networkID,
	}

	// A BaseTx that already passed syntactic verification.
	verifiedBaseTx := BaseTx{
		SyntacticallyVerified: true,
	}
	// Sanity check.
	require.NoError(t, verifiedBaseTx.SyntacticVerify(ctx))

	// A BaseTx that passes syntactic verification.
	validBaseTx := BaseTx{
		BaseTx: avax.BaseTx{
			NetworkID:    networkID,
			BlockchainID: chainID,
		},
	}
	// Sanity check.
	require.NoError(t, validBaseTx.SyntacticVerify(ctx))
	// Make sure we're not caching the verification result.
	require.False(t, validBaseTx.SyntacticallyVerified)

	// A BaseTx that fails syntactic verification.
	invalidBaseTx := BaseTx{}

	tests := []test{
		{
			name: "nil tx",
			txFunc: func(*gomock.Controller) *ExtendPermissionlessValidatorStakingTx {
				return nil
			},
			expectedErr: ErrNilTx,
		},
		{
			name: "already verified",
			txFunc: func(*gomock.Controller) *ExtendPermissionlessValidatorStakingTx {
				return &ExtendPermissionlessValidatorStakingTx{BaseTx: verifiedBaseTx}
			},
			expectedErr: nil,
		},
		{
			name: "invalid BaseTx",
			txFunc: func(*gomock.Controller) *ExtendPermissionlessValidatorStakingTx {
				return &ExtendPermissionlessValidatorStakingTx{
					// Set subnetID so we don't error on that check.
					Subnet: ids.GenerateTestID(),
					// Set NodeID so we don't error on that check.
					NodeID: ids.GenerateTestNodeID(),
					BaseTx: invalidBaseTx,
				}
			},
			expectedErr: avax.ErrWrongNetworkID,
		},
		{
			name: "empty nodeID",
			txFunc: func(*gomock.Controller) *ExtendPermissionlessValidatorStakingTx {
				return &ExtendPermissionlessValidatorStakingTx{
					BaseTx: validBaseTx,
					NodeID: ids.EmptyNodeID,
				}
			},
			expectedErr: errEmptyNodeID,
		},
		{
			name: "invalid StakerAuth",
			txFunc: func(ctrl *gomock.Controller) *ExtendPermissionlessValidatorStakingTx {
				// This SubnetAuth fails verification.
				invalidStakerAuth := verify.NewMockVerifiable(ctrl)
				invalidStakerAuth.EXPECT().Verify().Return(errInvalidSubnetAuth)
				return &ExtendPermissionlessValidatorStakingTx{
					// Set subnetID so we don't error on that check.
					Subnet: ids.GenerateTestID(),
					// Set NodeID so we don't error on that check.
					NodeID:     ids.GenerateTestNodeID(),
					BaseTx:     validBaseTx,
					StakerAuth: invalidStakerAuth,
				}
			},
			expectedErr: errInvalidSubnetAuth,
		},
		{
			name: "passes verification",
			txFunc: func(ctrl *gomock.Controller) *ExtendPermissionlessValidatorStakingTx {
				// This SubnetAuth passes verification.
				stakerAuth := verify.NewMockVerifiable(ctrl)
				stakerAuth.EXPECT().Verify().Return(nil)
				return &ExtendPermissionlessValidatorStakingTx{
					// Set subnetID so we don't error on that check.
					Subnet: ids.GenerateTestID(),
					// Set NodeID so we don't error on that check.
					NodeID:     ids.GenerateTestNodeID(),
					BaseTx:     validBaseTx,
					StakerAuth: stakerAuth,
				}
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			tx := tt.txFunc(ctrl)
			err := tx.SyntacticVerify(ctx)
			require.ErrorIs(err, tt.expectedErr)
			if tt.expectedErr == nil {
				require.True(tx.SyntacticallyVerified)
			}
		})
	}
}
