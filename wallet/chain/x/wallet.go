// Copyright (C) 2019-2024, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package x

import (
	"errors"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow/choices"
	"github.com/ava-labs/avalanchego/utils/timer/mockable"
	"github.com/ava-labs/avalanchego/version"
	"github.com/ava-labs/avalanchego/vms/avm"
	"github.com/ava-labs/avalanchego/vms/avm/config"
	"github.com/ava-labs/avalanchego/vms/avm/txs"
	"github.com/ava-labs/avalanchego/vms/avm/txs/fees"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/components/verify"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/subnet/primary/common"

	commonfees "github.com/ava-labs/avalanchego/vms/components/fees"
)

var (
	errNotAccepted = errors.New("not accepted")

	_ Wallet = (*wallet)(nil)
)

type Wallet interface {
	Context

	// Builder returns the builder that will be used to create the transactions.
	Builder() Builder

	// Signer returns the signer that will be used to sign the transactions.
	Signer() Signer

	// IssueBaseTx creates, signs, and issues a new simple value transfer.
	//
	// - [outputs] specifies all the recipients and amounts that should be sent
	//   from this transaction.
	IssueBaseTx(
		outputs []*avax.TransferableOutput,
		options ...common.Option,
	) (*txs.Tx, error)

	// IssueCreateAssetTx creates, signs, and issues a new asset.
	//
	// - [name] specifies a human readable name for this asset.
	// - [symbol] specifies a human readable abbreviation for this asset.
	// - [denomination] specifies how many times the asset can be split. For
	//   example, a denomination of [4] would mean that the smallest unit of the
	//   asset would be 0.001 units.
	// - [initialState] specifies the supported feature extensions for this
	//   asset as well as the initial outputs for the asset.
	IssueCreateAssetTx(
		name string,
		symbol string,
		denomination byte,
		initialState map[uint32][]verify.State,
		options ...common.Option,
	) (*txs.Tx, error)

	// IssueOperationTx creates, signs, and issues state changes on the UTXO
	// set. These state changes may be more complex than simple value transfers.
	//
	// - [operations] specifies the state changes to perform.
	IssueOperationTx(
		operations []*txs.Operation,
		options ...common.Option,
	) (*txs.Tx, error)

	// IssueOperationTxMintFT creates, signs, and issues a set of state changes
	// that mint new tokens for the requested assets.
	//
	// - [outputs] maps the assetID to the output that should be created for the
	//   asset.
	IssueOperationTxMintFT(
		outputs map[ids.ID]*secp256k1fx.TransferOutput,
		options ...common.Option,
	) (*txs.Tx, error)

	// IssueOperationTxMintNFT creates, signs, and issues a state change that
	// mints new NFTs for the requested asset.
	//
	// - [assetID] specifies the asset to mint the NFTs under.
	// - [payload] specifies the payload to provide each new NFT.
	// - [owners] specifies the new owners of each NFT.
	IssueOperationTxMintNFT(
		assetID ids.ID,
		payload []byte,
		owners []*secp256k1fx.OutputOwners,
		options ...common.Option,
	) (*txs.Tx, error)

	// IssueOperationTxMintProperty creates, signs, and issues a state change
	// that mints a new property for the requested asset.
	//
	// - [assetID] specifies the asset to mint the property under.
	// - [owner] specifies the new owner of the property.
	IssueOperationTxMintProperty(
		assetID ids.ID,
		owner *secp256k1fx.OutputOwners,
		options ...common.Option,
	) (*txs.Tx, error)

	// IssueOperationTxBurnProperty creates, signs, and issues state changes
	// that burns all the properties of the requested asset.
	//
	// - [assetID] specifies the asset to burn the property of.
	IssueOperationTxBurnProperty(
		assetID ids.ID,
		options ...common.Option,
	) (*txs.Tx, error)

	// IssueImportTx creates, signs, and issues an import transaction that
	// attempts to consume all the available UTXOs and import the funds to [to].
	//
	// - [chainID] specifies the chain to be importing funds from.
	// - [to] specifies where to send the imported funds to.
	IssueImportTx(
		chainID ids.ID,
		to *secp256k1fx.OutputOwners,
		options ...common.Option,
	) (*txs.Tx, error)

	// IssueExportTx creates, signs, and issues an export transaction that
	// attempts to send all the provided [outputs] to the requested [chainID].
	//
	// - [chainID] specifies the chain to be exporting the funds to.
	// - [outputs] specifies the outputs to send to the [chainID].
	IssueExportTx(
		chainID ids.ID,
		outputs []*avax.TransferableOutput,
		options ...common.Option,
	) (*txs.Tx, error)

	// IssueUnsignedTx signs and issues the unsigned tx.
	IssueUnsignedTx(
		utx txs.UnsignedTx,
		options ...common.Option,
	) (*txs.Tx, error)

	// IssueTx issues the signed tx.
	IssueTx(
		tx *txs.Tx,
		options ...common.Option,
	) error
}

func NewWallet(
	builder Builder,
	signer Signer,
	client avm.Client,
	backend Backend,
) Wallet {
	return &wallet{
		Backend: backend,
		builder: builder,
		signer:  signer,
		client:  client,
	}
}

type wallet struct {
	Backend
	signer Signer
	client avm.Client

	isEForkActive      bool
	builder            Builder
	unitFees, unitCaps commonfees.Dimensions
}

func (w *wallet) Builder() Builder {
	return w.builder
}

func (w *wallet) Signer() Signer {
	return w.signer
}

func (w *wallet) IssueBaseTx(
	outputs []*avax.TransferableOutput,
	options ...common.Option,
) (*txs.Tx, error) {
	w.refreshFork(options...)

	var (
		feesMan = commonfees.NewManager(w.unitFees)
		feeCalc = &fees.Calculator{
			IsEUpgradeActive: w.isEForkActive,
			Config: &config.Config{
				TxFee: w.BaseTxFee(),
			},
			FeeManager:       feesMan,
			ConsumedUnitsCap: w.unitCaps,
			Codec:            Parser.Codec(),
		}
	)

	utx, err := w.builder.NewBaseTx(outputs, feeCalc, options...)
	if err != nil {
		return nil, err
	}
	return w.IssueUnsignedTx(utx, options...)
}

func (w *wallet) IssueCreateAssetTx(
	name string,
	symbol string,
	denomination byte,
	initialState map[uint32][]verify.State,
	options ...common.Option,
) (*txs.Tx, error) {
	w.refreshFork(options...)

	var (
		feesMan = commonfees.NewManager(w.unitFees)
		feeCalc = &fees.Calculator{
			IsEUpgradeActive: w.isEForkActive,
			Config: &config.Config{
				TxFee: w.BaseTxFee(),
			},
			FeeManager:       feesMan,
			ConsumedUnitsCap: w.unitCaps,
			Codec:            Parser.Codec(),
		}
	)

	utx, err := w.builder.NewCreateAssetTx(name, symbol, denomination, initialState, feeCalc, options...)
	if err != nil {
		return nil, err
	}
	return w.IssueUnsignedTx(utx, options...)
}

func (w *wallet) IssueOperationTx(
	operations []*txs.Operation,
	options ...common.Option,
) (*txs.Tx, error) {
	w.refreshFork(options...)

	var (
		feesMan = commonfees.NewManager(w.unitFees)
		feeCalc = &fees.Calculator{
			IsEUpgradeActive: w.isEForkActive,
			Config: &config.Config{
				TxFee: w.BaseTxFee(),
			},
			FeeManager:       feesMan,
			ConsumedUnitsCap: w.unitCaps,
			Codec:            Parser.Codec(),
		}
	)

	utx, err := w.builder.NewOperationTx(operations, feeCalc, options...)
	if err != nil {
		return nil, err
	}
	return w.IssueUnsignedTx(utx, options...)
}

func (w *wallet) IssueOperationTxMintFT(
	outputs map[ids.ID]*secp256k1fx.TransferOutput,
	options ...common.Option,
) (*txs.Tx, error) {
	w.refreshFork(options...)

	var (
		feesMan = commonfees.NewManager(w.unitFees)
		feeCalc = &fees.Calculator{
			IsEUpgradeActive: w.isEForkActive,
			Config: &config.Config{
				TxFee: w.BaseTxFee(),
			},
			FeeManager:       feesMan,
			ConsumedUnitsCap: w.unitCaps,
			Codec:            Parser.Codec(),
		}
	)

	utx, err := w.builder.NewOperationTxMintFT(outputs, feeCalc, options...)
	if err != nil {
		return nil, err
	}
	return w.IssueUnsignedTx(utx, options...)
}

func (w *wallet) IssueOperationTxMintNFT(
	assetID ids.ID,
	payload []byte,
	owners []*secp256k1fx.OutputOwners,
	options ...common.Option,
) (*txs.Tx, error) {
	w.refreshFork(options...)

	var (
		feesMan = commonfees.NewManager(w.unitFees)
		feeCalc = &fees.Calculator{
			IsEUpgradeActive: w.isEForkActive,
			Config: &config.Config{
				TxFee: w.BaseTxFee(),
			},
			FeeManager:       feesMan,
			ConsumedUnitsCap: w.unitCaps,
			Codec:            Parser.Codec(),
		}
	)

	utx, err := w.builder.NewOperationTxMintNFT(assetID, payload, owners, feeCalc, options...)
	if err != nil {
		return nil, err
	}
	return w.IssueUnsignedTx(utx, options...)
}

func (w *wallet) IssueOperationTxMintProperty(
	assetID ids.ID,
	owner *secp256k1fx.OutputOwners,
	options ...common.Option,
) (*txs.Tx, error) {
	w.refreshFork(options...)

	var (
		feesMan = commonfees.NewManager(w.unitFees)
		feeCalc = &fees.Calculator{
			IsEUpgradeActive: w.isEForkActive,
			Config: &config.Config{
				TxFee: w.BaseTxFee(),
			},
			FeeManager:       feesMan,
			ConsumedUnitsCap: w.unitCaps,
			Codec:            Parser.Codec(),
		}
	)

	utx, err := w.builder.NewOperationTxMintProperty(assetID, owner, feeCalc, options...)
	if err != nil {
		return nil, err
	}
	return w.IssueUnsignedTx(utx, options...)
}

func (w *wallet) IssueOperationTxBurnProperty(
	assetID ids.ID,
	options ...common.Option,
) (*txs.Tx, error) {
	w.refreshFork(options...)

	var (
		feesMan = commonfees.NewManager(w.unitFees)
		feeCalc = &fees.Calculator{
			IsEUpgradeActive: w.isEForkActive,
			Config: &config.Config{
				TxFee: w.BaseTxFee(),
			},
			FeeManager:       feesMan,
			ConsumedUnitsCap: w.unitCaps,
			Codec:            Parser.Codec(),
		}
	)

	utx, err := w.builder.NewOperationTxBurnProperty(assetID, feeCalc, options...)
	if err != nil {
		return nil, err
	}
	return w.IssueUnsignedTx(utx, options...)
}

func (w *wallet) IssueImportTx(
	chainID ids.ID,
	to *secp256k1fx.OutputOwners,
	options ...common.Option,
) (*txs.Tx, error) {
	w.refreshFork(options...)

	var (
		feesMan = commonfees.NewManager(w.unitFees)
		feeCalc = &fees.Calculator{
			IsEUpgradeActive: w.isEForkActive,
			Config: &config.Config{
				TxFee: w.BaseTxFee(),
			},
			FeeManager:       feesMan,
			ConsumedUnitsCap: w.unitCaps,
			Codec:            Parser.Codec(),
		}
	)

	utx, err := w.builder.NewImportTx(chainID, to, feeCalc, options...)
	if err != nil {
		return nil, err
	}
	return w.IssueUnsignedTx(utx, options...)
}

func (w *wallet) IssueExportTx(
	chainID ids.ID,
	outputs []*avax.TransferableOutput,
	options ...common.Option,
) (*txs.Tx, error) {
	w.refreshFork(options...)

	var (
		feesMan = commonfees.NewManager(w.unitFees)
		feeCalc = &fees.Calculator{
			IsEUpgradeActive: w.isEForkActive,
			Config: &config.Config{
				TxFee: w.BaseTxFee(),
			},
			FeeManager:       feesMan,
			ConsumedUnitsCap: w.unitCaps,
			Codec:            Parser.Codec(),
		}
	)

	utx, err := w.builder.NewExportTx(chainID, outputs, feeCalc, options...)
	if err != nil {
		return nil, err
	}
	return w.IssueUnsignedTx(utx, options...)
}

func (w *wallet) IssueUnsignedTx(
	utx txs.UnsignedTx,
	options ...common.Option,
) (*txs.Tx, error) {
	ops := common.NewOptions(options)
	ctx := ops.Context()
	tx, err := SignUnsigned(ctx, w.signer, utx)
	if err != nil {
		return nil, err
	}

	return tx, w.IssueTx(tx, options...)
}

func (w *wallet) IssueTx(
	tx *txs.Tx,
	options ...common.Option,
) error {
	ops := common.NewOptions(options)
	ctx := ops.Context()
	txID, err := w.client.IssueTx(ctx, tx.Bytes())
	if err != nil {
		return err
	}

	if f := ops.PostIssuanceFunc(); f != nil {
		f(txID)
	}

	if ops.AssumeDecided() {
		return w.Backend.AcceptTx(ctx, tx)
	}

	txStatus, err := w.client.ConfirmTx(ctx, txID, ops.PollFrequency())
	if err != nil {
		return err
	}

	if err := w.Backend.AcceptTx(ctx, tx); err != nil {
		return err
	}

	if txStatus != choices.Accepted {
		return errNotAccepted
	}
	return nil
}

func (w *wallet) refreshFork(_ ...common.Option) {
	if w.isEForkActive {
		// E fork enables dinamic fees and it is active
		// not need to recheck
		return
	}

	// ops       = common.NewOptions(options)
	// ctx       = ops.Context()
	eForkTime := version.GetEUpgradeTime(w.NetworkID())

	// TODO ABENEGIA: consider introducing this method in X-chain as well
	// chainTime, err := w.client.GetTimestamp(ctx)
	// if err != nil {
	// 	return err
	// }
	chainTime := mockable.MaxTime // assume fork is already active

	w.isEForkActive = !chainTime.Before(eForkTime)
	if w.isEForkActive {
		w.unitFees = config.EUpgradeDynamicFeesConfig.UnitFees
		w.unitCaps = config.EUpgradeDynamicFeesConfig.BlockUnitsCap
	} else {
		w.unitFees = config.PreEUpgradeDynamicFeesConfig.UnitFees
		w.unitCaps = config.PreEUpgradeDynamicFeesConfig.BlockUnitsCap
	}
}
