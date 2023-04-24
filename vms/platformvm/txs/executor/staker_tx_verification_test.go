// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package executor

import (
	"math"
	"math/rand"
	"testing"
	"time"

	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/timer/mockable"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/vms/platformvm/config"
	"github.com/ava-labs/avalanchego/vms/platformvm/fx"
	"github.com/ava-labs/avalanchego/vms/platformvm/state"
	"github.com/ava-labs/avalanchego/vms/platformvm/status"
	"github.com/ava-labs/avalanchego/vms/platformvm/txs"
	"github.com/ava-labs/avalanchego/vms/platformvm/utxo"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

func TestVerifyAddPermissionlessValidatorTx(t *testing.T) {
	type test struct {
		name        string
		backendF    func(*gomock.Controller) *Backend
		stateF      func(*gomock.Controller) state.Chain
		sTxF        func() *txs.Tx
		txF         func() *txs.AddPermissionlessValidatorTx
		expectedErr error
	}

	var (
		subnetID            = ids.GenerateTestID()
		customAssetID       = ids.GenerateTestID()
		unsignedTransformTx = &txs.TransformSubnetTx{
			AssetID:           customAssetID,
			MinValidatorStake: 1,
			MaxValidatorStake: 2,
			MinStakeDuration:  3,
			MaxStakeDuration:  4,
			MinDelegationFee:  5,
		}
		transformTx = txs.Tx{
			Unsigned: unsignedTransformTx,
			Creds:    []verify.Verifiable{},
		}
		// This tx already passed syntactic verification.
		verifiedTx = txs.AddPermissionlessValidatorTx{
			BaseTx: txs.BaseTx{
				SyntacticallyVerified: true,
				BaseTx: avax.BaseTx{
					NetworkID:    1,
					BlockchainID: ids.GenerateTestID(),
					Outs:         []*avax.TransferableOutput{},
					Ins:          []*avax.TransferableInput{},
				},
			},
			Validator: txs.Validator{
				NodeID: ids.GenerateTestNodeID(),
				Start:  uint64(0),
				End:    uint64(unsignedTransformTx.MinStakeDuration),
				Wght:   unsignedTransformTx.MinValidatorStake,
			},
			Subnet: subnetID,
			StakeOuts: []*avax.TransferableOutput{
				{
					Asset: avax.Asset{
						ID: customAssetID,
					},
				},
			},
			ValidatorRewardsOwner: &secp256k1fx.OutputOwners{
				Addrs:     []ids.ShortID{ids.GenerateTestShortID()},
				Threshold: 1,
			},
			DelegatorRewardsOwner: &secp256k1fx.OutputOwners{
				Addrs:     []ids.ShortID{ids.GenerateTestShortID()},
				Threshold: 1,
			},
			DelegationShares: 20_000,
		}
		verifiedSignedTx = txs.Tx{
			Unsigned: &verifiedTx,
			Creds:    []verify.Verifiable{},
		}
	)
	verifiedSignedTx.SetBytes([]byte{1}, []byte{2})

	tests := []test{
		{
			name: "fail syntactic verification",
			backendF: func(*gomock.Controller) *Backend {
				return &Backend{
					Ctx: snow.DefaultContextTest(),
					Config: &config.Config{
						ContinuousStakingTime: time.Time{}, // activate latest fork
					},
				}
			},
			stateF: func(*gomock.Controller) state.Chain {
				return nil
			},
			sTxF: func() *txs.Tx {
				return nil
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				return nil
			},
			expectedErr: txs.ErrNilSignedTx,
		},
		{
			name: "not bootstrapped",
			backendF: func(*gomock.Controller) *Backend {
				return &Backend{
					Ctx: snow.DefaultContextTest(),
					Config: &config.Config{
						ContinuousStakingTime: time.Time{}, // activate latest fork
					},
					Bootstrapped: &utils.Atomic[bool]{},
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				return nil
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				return nil
			},
			expectedErr: nil,
		},
		{
			name: "weight too low",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)
				return &Backend{
					Ctx: snow.DefaultContextTest(),
					Config: &config.Config{
						ContinuousStakingTime: time.Time{}, // activate latest fork
					},
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				state.EXPECT().GetSubnetTransformation(subnetID).Return(&transformTx, nil)
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				tx.Validator.Wght = unsignedTransformTx.MinValidatorStake - 1
				return &tx
			},
			expectedErr: ErrWeightTooSmall,
		},
		{
			name: "weight too high",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)
				return &Backend{
					Ctx: snow.DefaultContextTest(),
					Config: &config.Config{
						ContinuousStakingTime: time.Time{}, // activate latest fork
					},
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				state.EXPECT().GetSubnetTransformation(subnetID).Return(&transformTx, nil)
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				tx.Validator.Wght = unsignedTransformTx.MaxValidatorStake + 1
				return &tx
			},
			expectedErr: ErrWeightTooLarge,
		},
		{
			name: "insufficient delegation fee",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)
				return &Backend{
					Ctx: snow.DefaultContextTest(),
					Config: &config.Config{
						ContinuousStakingTime: time.Time{}, // activate latest fork
					},
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				state.EXPECT().GetSubnetTransformation(subnetID).Return(&transformTx, nil)
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				tx.Validator.Wght = unsignedTransformTx.MaxValidatorStake
				tx.DelegationShares = unsignedTransformTx.MinDelegationFee - 1
				return &tx
			},
			expectedErr: ErrInsufficientDelegationFee,
		},
		{
			name: "duration too short",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)
				return &Backend{
					Ctx: snow.DefaultContextTest(),
					Config: &config.Config{
						ContinuousStakingTime: time.Time{}, // activate latest fork
					},
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				state.EXPECT().GetSubnetTransformation(subnetID).Return(&transformTx, nil)
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				tx.Validator.Wght = unsignedTransformTx.MaxValidatorStake
				tx.DelegationShares = unsignedTransformTx.MinDelegationFee
				// Note the duration is 1 less than the minimum
				tx.Validator.Start = 0
				tx.Validator.End = uint64(unsignedTransformTx.MinStakeDuration) - 1
				return &tx
			},
			expectedErr: ErrStakeTooShort,
		},
		{
			name: "duration too long",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)
				return &Backend{
					Ctx: snow.DefaultContextTest(),
					Config: &config.Config{
						ContinuousStakingTime: time.Time{}, // activate latest fork
					},
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				state.EXPECT().GetSubnetTransformation(subnetID).Return(&transformTx, nil)
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				tx.Validator.Wght = unsignedTransformTx.MaxValidatorStake
				tx.DelegationShares = unsignedTransformTx.MinDelegationFee
				// Note the duration is more than the maximum
				tx.Validator.Start = 0
				tx.Validator.End = 1 + uint64(unsignedTransformTx.MaxStakeDuration)
				return &tx
			},
			expectedErr: ErrStakeTooLong,
		},
		{
			name: "wrong assetID",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)
				return &Backend{
					Ctx: snow.DefaultContextTest(),
					Config: &config.Config{
						ContinuousStakingTime: time.Time{}, // activate latest fork
					},
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				state.EXPECT().GetSubnetTransformation(subnetID).Return(&transformTx, nil)
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				tx.StakeOuts = []*avax.TransferableOutput{
					{
						Asset: avax.Asset{
							ID: ids.GenerateTestID(),
						},
					},
				}
				return &tx
			},
			expectedErr: ErrWrongStakedAssetID,
		},
		{
			name: "duplicate validator",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)
				return &Backend{
					Ctx: snow.DefaultContextTest(),
					Config: &config.Config{
						ContinuousStakingTime: time.Time{}, // activate latest fork
					},
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				state.EXPECT().GetSubnetTransformation(subnetID).Return(&transformTx, nil)
				// State says validator exists
				state.EXPECT().GetCurrentValidator(subnetID, verifiedTx.NodeID()).Return(nil, nil)
				return state
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				return &verifiedTx
			},
			expectedErr: ErrDuplicateValidator,
		},
		{
			name: "validator not subset of primary network validator",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)
				return &Backend{
					Ctx: snow.DefaultContextTest(),
					Config: &config.Config{
						ContinuousStakingTime: time.Time{}, // activate latest fork
					},
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				mockState := state.NewMockChain(ctrl)
				mockState.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				mockState.EXPECT().GetSubnetTransformation(subnetID).Return(&transformTx, nil)
				mockState.EXPECT().GetCurrentValidator(subnetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				mockState.EXPECT().GetPendingValidator(subnetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				// Validator time isn't subset of primary network validator time
				primaryNetworkVdr := &state.Staker{
					StartTime:     verifiedTx.StartTime(),
					StakingPeriod: verifiedTx.StakingPeriod() - 1,
					EndTime:       verifiedTx.EndTime().Add(-1 * time.Second),
				}
				mockState.EXPECT().GetCurrentValidator(constants.PrimaryNetworkID, verifiedTx.NodeID()).Return(primaryNetworkVdr, nil)
				return mockState
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				return &verifiedTx
			},
			expectedErr: ErrValidatorSubset,
		},
		{
			name: "flow check fails",
			backendF: func(ctrl *gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)

				flowChecker := utxo.NewMockVerifier(ctrl)
				flowChecker.EXPECT().VerifySpend(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(ErrFlowCheckFailed)

				return &Backend{
					FlowChecker: flowChecker,
					Config: &config.Config{
						AddSubnetValidatorFee: 1,
						ContinuousStakingTime: time.Time{}, // activate latest fork,
					},
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				mockState := state.NewMockChain(ctrl)
				mockState.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				mockState.EXPECT().GetSubnetTransformation(subnetID).Return(&transformTx, nil)
				mockState.EXPECT().GetCurrentValidator(subnetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				mockState.EXPECT().GetPendingValidator(subnetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				primaryNetworkVdr := &state.Staker{
					StartTime:     verifiedTx.StartTime(),
					StakingPeriod: verifiedTx.StakingPeriod(),
					EndTime:       verifiedTx.EndTime(),
				}
				mockState.EXPECT().GetCurrentValidator(constants.PrimaryNetworkID, verifiedTx.NodeID()).Return(primaryNetworkVdr, nil)
				return mockState
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				return &verifiedTx
			},
			expectedErr: ErrFlowCheckFailed,
		},
		{
			name: "success",
			backendF: func(ctrl *gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)

				flowChecker := utxo.NewMockVerifier(ctrl)
				flowChecker.EXPECT().VerifySpend(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(nil)

				return &Backend{
					FlowChecker: flowChecker,
					Config: &config.Config{
						AddSubnetValidatorFee: 1,
						ContinuousStakingTime: time.Time{}, // activate latest fork,
					},
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				mockState := state.NewMockChain(ctrl)
				mockState.EXPECT().GetTimestamp().Return(time.Unix(0, 0))
				mockState.EXPECT().GetSubnetTransformation(subnetID).Return(&transformTx, nil)
				mockState.EXPECT().GetCurrentValidator(subnetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				mockState.EXPECT().GetPendingValidator(subnetID, verifiedTx.NodeID()).Return(nil, database.ErrNotFound)
				primaryNetworkVdr := &state.Staker{
					StartTime:     time.Unix(0, 0),
					StakingPeriod: state.StakerMaxDuration,
					EndTime:       mockable.MaxTime,
				}
				mockState.EXPECT().GetCurrentValidator(constants.PrimaryNetworkID, verifiedTx.NodeID()).Return(primaryNetworkVdr, nil)
				return mockState
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.AddPermissionlessValidatorTx {
				return &verifiedTx
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var (
				backend = tt.backendF(ctrl)
				state   = tt.stateF(ctrl)
				sTx     = tt.sTxF()
				tx      = tt.txF()
			)

			err := verifyAddPermissionlessValidatorTx(backend, state, sTx, tx)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}

func TestGetValidatorRules(t *testing.T) {
	type test struct {
		name          string
		subnetID      ids.ID
		backend       *Backend
		chainStateF   func(*gomock.Controller) state.Chain
		expectedRules *addValidatorRules
		expectedErr   error
	}

	var (
		config = &config.Config{
			MinValidatorStake: 1,
			MaxValidatorStake: 2,
			MinStakeDuration:  time.Second,
			MaxStakeDuration:  2 * time.Second,
			MinDelegationFee:  1337,
		}
		avaxAssetID   = ids.GenerateTestID()
		customAssetID = ids.GenerateTestID()
		subnetID      = ids.GenerateTestID()
	)

	tests := []test{
		{
			name:     "primary network",
			subnetID: constants.PrimaryNetworkID,
			backend: &Backend{
				Config: config,
				Ctx: &snow.Context{
					AVAXAssetID: avaxAssetID,
				},
			},
			chainStateF: func(*gomock.Controller) state.Chain {
				return nil
			},
			expectedRules: &addValidatorRules{
				assetID:           avaxAssetID,
				minValidatorStake: config.MinValidatorStake,
				maxValidatorStake: config.MaxValidatorStake,
				minStakeDuration:  config.MinStakeDuration,
				maxStakeDuration:  config.MaxStakeDuration,
				minDelegationFee:  config.MinDelegationFee,
			},
		},
		{
			name:     "can't get subnet transformation",
			subnetID: subnetID,
			backend:  nil,
			chainStateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetSubnetTransformation(subnetID).Return(nil, errTest)
				return state
			},
			expectedRules: &addValidatorRules{},
			expectedErr:   errTest,
		},
		{
			name:     "invalid transformation tx",
			subnetID: subnetID,
			backend:  nil,
			chainStateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				tx := &txs.Tx{
					Unsigned: &txs.AddDelegatorTx{},
				}
				state.EXPECT().GetSubnetTransformation(subnetID).Return(tx, nil)
				return state
			},
			expectedRules: &addValidatorRules{},
			expectedErr:   ErrIsNotTransformSubnetTx,
		},
		{
			name:     "subnet",
			subnetID: subnetID,
			backend:  nil,
			chainStateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				tx := &txs.Tx{
					Unsigned: &txs.TransformSubnetTx{
						AssetID:           customAssetID,
						MinValidatorStake: config.MinValidatorStake,
						MaxValidatorStake: config.MaxValidatorStake,
						MinStakeDuration:  1337,
						MaxStakeDuration:  42,
						MinDelegationFee:  config.MinDelegationFee,
					},
				}
				state.EXPECT().GetSubnetTransformation(subnetID).Return(tx, nil)
				return state
			},
			expectedRules: &addValidatorRules{
				assetID:           customAssetID,
				minValidatorStake: config.MinValidatorStake,
				maxValidatorStake: config.MaxValidatorStake,
				minStakeDuration:  time.Duration(1337) * time.Second,
				maxStakeDuration:  time.Duration(42) * time.Second,
				minDelegationFee:  config.MinDelegationFee,
			},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			chainState := tt.chainStateF(ctrl)
			rules, err := getValidatorRules(tt.backend, chainState, tt.subnetID)
			if tt.expectedErr != nil {
				require.ErrorIs(err, tt.expectedErr)
				return
			}
			require.NoError(err)
			require.Equal(tt.expectedRules, rules)
		})
	}
}

func TestGetDelegatorRules(t *testing.T) {
	type test struct {
		name          string
		subnetID      ids.ID
		backend       *Backend
		chainStateF   func(*gomock.Controller) state.Chain
		expectedRules *addDelegatorRules
		expectedErr   error
	}
	var (
		config = &config.Config{
			MinDelegatorStake: 1,
			MaxValidatorStake: 2,
			MinStakeDuration:  time.Second,
			MaxStakeDuration:  2 * time.Second,
		}
		avaxAssetID   = ids.GenerateTestID()
		customAssetID = ids.GenerateTestID()
		subnetID      = ids.GenerateTestID()
	)
	tests := []test{
		{
			name:     "primary network",
			subnetID: constants.PrimaryNetworkID,
			backend: &Backend{
				Config: config,
				Ctx: &snow.Context{
					AVAXAssetID: avaxAssetID,
				},
			},
			chainStateF: func(*gomock.Controller) state.Chain {
				return nil
			},
			expectedRules: &addDelegatorRules{
				assetID:                  avaxAssetID,
				minDelegatorStake:        config.MinDelegatorStake,
				maxValidatorStake:        config.MaxValidatorStake,
				minStakeDuration:         config.MinStakeDuration,
				maxStakeDuration:         config.MaxStakeDuration,
				maxValidatorWeightFactor: MaxValidatorWeightFactor,
			},
		},
		{
			name:     "can't get subnet transformation",
			subnetID: subnetID,
			backend:  nil,
			chainStateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				state.EXPECT().GetSubnetTransformation(subnetID).Return(nil, errTest)
				return state
			},
			expectedRules: &addDelegatorRules{},
			expectedErr:   errTest,
		},
		{
			name:     "invalid transformation tx",
			subnetID: subnetID,
			backend:  nil,
			chainStateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				tx := &txs.Tx{
					Unsigned: &txs.AddDelegatorTx{},
				}
				state.EXPECT().GetSubnetTransformation(subnetID).Return(tx, nil)
				return state
			},
			expectedRules: &addDelegatorRules{},
			expectedErr:   ErrIsNotTransformSubnetTx,
		},
		{
			name:     "subnet",
			subnetID: subnetID,
			backend:  nil,
			chainStateF: func(ctrl *gomock.Controller) state.Chain {
				state := state.NewMockChain(ctrl)
				tx := &txs.Tx{
					Unsigned: &txs.TransformSubnetTx{
						AssetID:                  customAssetID,
						MinDelegatorStake:        config.MinDelegatorStake,
						MinValidatorStake:        config.MinValidatorStake,
						MaxValidatorStake:        config.MaxValidatorStake,
						MinStakeDuration:         1337,
						MaxStakeDuration:         42,
						MinDelegationFee:         config.MinDelegationFee,
						MaxValidatorWeightFactor: 21,
					},
				}
				state.EXPECT().GetSubnetTransformation(subnetID).Return(tx, nil)
				return state
			},
			expectedRules: &addDelegatorRules{
				assetID:                  customAssetID,
				minDelegatorStake:        config.MinDelegatorStake,
				maxValidatorStake:        config.MaxValidatorStake,
				minStakeDuration:         time.Duration(1337) * time.Second,
				maxStakeDuration:         time.Duration(42) * time.Second,
				maxValidatorWeightFactor: 21,
			},
			expectedErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require := require.New(t)
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			chainState := tt.chainStateF(ctrl)
			rules, err := getDelegatorRules(tt.backend, chainState, tt.subnetID)
			if tt.expectedErr != nil {
				require.ErrorIs(err, tt.expectedErr)
				return
			}
			require.NoError(err)
			require.Equal(tt.expectedRules, rules)
		})
	}
}

func TestVerifyExtendPermissionlessValidatorTx(t *testing.T) {
	type test struct {
		name        string
		backendF    func(*gomock.Controller) *Backend
		stateF      func(*gomock.Controller) state.Chain
		sTxF        func() *txs.Tx
		txF         func() *txs.ExtendPermissionlessValidatorStakingTx
		expectedErr error
	}

	var (
		continuousStakingFork = time.Now()
		rndStartTime          = time.Unix(rand.Int63(), 0)                     // #nosec G404
		rndStakingPeriod      = time.Duration(math.Abs(float64(rand.Int63()))) // #nosec G404
		rndNextTime           = rndStartTime.Add(rndStakingPeriod)
		validatorToExtend     = &state.Staker{
			TxID:          ids.GenerateTestID(),
			NodeID:        ids.GenerateTestNodeID(),
			SubnetID:      ids.GenerateTestID(),
			StartTime:     rndStartTime,
			StakingPeriod: rndStakingPeriod,
			EndTime:       rndNextTime,
			NextTime:      rndNextTime,
			Priority:      txs.SubnetPermissionlessValidatorCurrentPriority,
		}

		// This tx already passed syntactic verification.
		verifiedTx = txs.ExtendPermissionlessValidatorStakingTx{
			BaseTx: txs.BaseTx{
				SyntacticallyVerified: true,
				BaseTx: avax.BaseTx{
					NetworkID:    1,
					BlockchainID: ids.GenerateTestID(),
					Outs:         []*avax.TransferableOutput{},
					Ins:          []*avax.TransferableInput{},
				},
			},
			NodeID:     validatorToExtend.NodeID,
			Subnet:     validatorToExtend.SubnetID,
			StakerAuth: nil,
		}
		verifiedSignedTx = txs.Tx{
			Unsigned: &verifiedTx,
			Creds: []verify.Verifiable{
				&secp256k1fx.Credential{
					Sigs: make([][65]byte, 1),
				},
				&secp256k1fx.Credential{
					Sigs: make([][65]byte, 1),
				},
			},
		}
	)
	verifiedSignedTx.SetBytes([]byte{1}, []byte{2})

	tests := []test{
		{
			name: "fail syntactic verification",
			backendF: func(*gomock.Controller) *Backend {
				return &Backend{
					Ctx: snow.DefaultContextTest(),
					Config: &config.Config{
						ContinuousStakingTime:            continuousStakingFork,
						ExtendPermissionlessValidatorFee: 10,
					},
				}
			},
			stateF: func(*gomock.Controller) state.Chain {
				return nil
			},
			sTxF: func() *txs.Tx {
				return nil
			},
			txF: func() *txs.ExtendPermissionlessValidatorStakingTx {
				return nil
			},
			expectedErr: txs.ErrNilTx,
		},
		{
			name: "pre continuous staking fork",
			backendF: func(*gomock.Controller) *Backend {
				return &Backend{
					Ctx: snow.DefaultContextTest(),
					Config: &config.Config{
						ContinuousStakingTime:            continuousStakingFork,
						ExtendPermissionlessValidatorFee: 10,
					},
					Bootstrapped: &utils.Atomic[bool]{},
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				mockState := state.NewMockChain(ctrl)
				mockState.EXPECT().GetTimestamp().Return(continuousStakingFork.Add(-1 * time.Second))
				return mockState
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.ExtendPermissionlessValidatorStakingTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				return &tx
			},
			expectedErr: ErrTxNotAllowedInCurrentFork,
		},
		{
			name: "missing validator to extend",
			backendF: func(*gomock.Controller) *Backend {
				return &Backend{
					Ctx: snow.DefaultContextTest(),
					Config: &config.Config{
						ContinuousStakingTime:            continuousStakingFork,
						ExtendPermissionlessValidatorFee: 10,
					},
					Bootstrapped: &utils.Atomic[bool]{},
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				mockState := state.NewMockChain(ctrl)
				mockState.EXPECT().GetTimestamp().Return(continuousStakingFork)
				mockState.EXPECT().GetCurrentValidator(validatorToExtend.SubnetID, validatorToExtend.NodeID).
					Return(nil, database.ErrNotFound)
				return mockState
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.ExtendPermissionlessValidatorStakingTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				return &tx
			},
			expectedErr: database.ErrNotFound,
		},
		{
			name: "not bootstrapped",
			backendF: func(*gomock.Controller) *Backend {
				return &Backend{
					Ctx: snow.DefaultContextTest(),
					Config: &config.Config{
						ContinuousStakingTime:            continuousStakingFork,
						ExtendPermissionlessValidatorFee: 10,
					},
					Bootstrapped: &utils.Atomic[bool]{},
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				mockState := state.NewMockChain(ctrl)
				mockState.EXPECT().GetTimestamp().Return(continuousStakingFork)
				mockState.EXPECT().GetCurrentValidator(validatorToExtend.SubnetID, validatorToExtend.NodeID).
					Return(validatorToExtend, nil)
				return mockState
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.ExtendPermissionlessValidatorStakingTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				return &tx
			},
			expectedErr: nil,
		},
		{
			name: "forbid extending permissioned validators",
			backendF: func(*gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)
				return &Backend{
					Ctx: snow.DefaultContextTest(),
					Config: &config.Config{
						ContinuousStakingTime: continuousStakingFork,
					},
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				permissionedVal := *validatorToExtend
				permissionedVal.Priority = txs.SubnetPermissionedValidatorCurrentPriority

				mockState := state.NewMockChain(ctrl)
				mockState.EXPECT().GetTimestamp().Return(continuousStakingFork)
				mockState.EXPECT().GetCurrentValidator(validatorToExtend.SubnetID, validatorToExtend.NodeID).
					Return(&permissionedVal, nil)
				return mockState
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.ExtendPermissionlessValidatorStakingTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				return &tx
			},
			expectedErr: ErrCannotExtendPermissionedValidator,
		},
		{
			name: "missing credentials",
			backendF: func(ctrl *gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)
				return &Backend{
					Config: &config.Config{
						ContinuousStakingTime:            continuousStakingFork,
						ExtendPermissionlessValidatorFee: 10,
					},
					Ctx:          snow.DefaultContextTest(),
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				mockState := state.NewMockChain(ctrl)
				mockState.EXPECT().GetTimestamp().Return(continuousStakingFork)
				mockState.EXPECT().GetCurrentValidator(
					validatorToExtend.SubnetID,
					validatorToExtend.NodeID,
				).Return(validatorToExtend, nil)
				return mockState
			},
			sTxF: func() *txs.Tx {
				res := verifiedSignedTx
				res.Creds = nil
				return &res
			},
			txF: func() *txs.ExtendPermissionlessValidatorStakingTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				return &tx
			},
			expectedErr: errWrongNumberOfCredentials,
		},
		{
			name: "missing permission to extend staking",
			backendF: func(ctrl *gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)

				fx := fx.NewMockFx(ctrl)
				fx.EXPECT().VerifyPermission(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(ErrUnauthorizedStakingExtension)

				return &Backend{
					Config: &config.Config{
						ContinuousStakingTime:            continuousStakingFork,
						ExtendPermissionlessValidatorFee: 10,
					},
					Ctx:          snow.DefaultContextTest(),
					Fx:           fx,
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				mockState := state.NewMockChain(ctrl)
				mockState.EXPECT().GetTimestamp().Return(continuousStakingFork)
				mockState.EXPECT().GetCurrentValidator(
					validatorToExtend.SubnetID,
					validatorToExtend.NodeID,
				).Return(validatorToExtend, nil)

				addValTx := &txs.AddPermissionlessValidatorTx{
					Validator: txs.Validator{
						NodeID: validatorToExtend.NodeID,
						Start:  uint64(validatorToExtend.StartTime.Unix()),
						End:    uint64(validatorToExtend.EndTime.Unix()),
						Wght:   validatorToExtend.Weight,
					},
					Subnet: validatorToExtend.SubnetID,
				}
				signedAddValTx := txs.Tx{
					Unsigned: addValTx,
					Creds:    []verify.Verifiable{},
				}
				signedAddValTx.SetBytes([]byte{1}, []byte{2})

				mockState.EXPECT().GetTx(validatorToExtend.TxID).Return(&signedAddValTx, status.Committed, nil)
				return mockState
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.ExtendPermissionlessValidatorStakingTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				return &tx
			},
			expectedErr: ErrUnauthorizedStakingExtension,
		},
		{
			name: "success",
			backendF: func(ctrl *gomock.Controller) *Backend {
				bootstrapped := &utils.Atomic[bool]{}
				bootstrapped.Set(true)

				flowChecker := utxo.NewMockVerifier(ctrl)
				flowChecker.EXPECT().VerifySpend(
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
					gomock.Any(),
				).Return(nil)

				fx := fx.NewMockFx(ctrl)
				fx.EXPECT().VerifyPermission(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

				return &Backend{
					FlowChecker: flowChecker,
					Config: &config.Config{
						ContinuousStakingTime:            continuousStakingFork,
						ExtendPermissionlessValidatorFee: 10,
					},
					Ctx:          snow.DefaultContextTest(),
					Fx:           fx,
					Bootstrapped: bootstrapped,
				}
			},
			stateF: func(ctrl *gomock.Controller) state.Chain {
				mockState := state.NewMockChain(ctrl)
				mockState.EXPECT().GetTimestamp().Return(continuousStakingFork)
				mockState.EXPECT().GetCurrentValidator(
					validatorToExtend.SubnetID,
					validatorToExtend.NodeID,
				).Return(validatorToExtend, nil)

				addValTx := &txs.AddPermissionlessValidatorTx{
					Validator: txs.Validator{
						NodeID: validatorToExtend.NodeID,
						Start:  uint64(validatorToExtend.StartTime.Unix()),
						End:    uint64(validatorToExtend.EndTime.Unix()),
						Wght:   validatorToExtend.Weight,
					},
					Subnet: validatorToExtend.SubnetID,
				}
				signedAddValTx := txs.Tx{
					Unsigned: addValTx,
					Creds:    []verify.Verifiable{},
				}
				signedAddValTx.SetBytes([]byte{1}, []byte{2})

				mockState.EXPECT().GetTx(validatorToExtend.TxID).Return(&signedAddValTx, status.Committed, nil)
				return mockState
			},
			sTxF: func() *txs.Tx {
				return &verifiedSignedTx
			},
			txF: func() *txs.ExtendPermissionlessValidatorStakingTx {
				tx := verifiedTx // Note that this copies [verifiedTx]
				return &tx
			},
			expectedErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			var (
				backend = tt.backendF(ctrl)
				state   = tt.stateF(ctrl)
				sTx     = tt.sTxF()
				tx      = tt.txF()
			)

			_, err := verifyExtendPermissionlessValidatorTx(backend, state, sTx, tx)
			require.ErrorIs(t, err, tt.expectedErr)
		})
	}
}
