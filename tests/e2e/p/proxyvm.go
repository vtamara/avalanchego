// Copyright (C) 2019-2023, Ava Labs, Inc. All rights reserved.
// See the file LICENSE for licensing terms.

package p

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	ginkgo "github.com/onsi/ginkgo/v2"

	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/config"
	"github.com/ava-labs/avalanchego/tests"
	"github.com/ava-labs/avalanchego/tests/e2e"
	"github.com/ava-labs/avalanchego/tests/fixture/testnet"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/units"
	"github.com/ava-labs/avalanchego/vms/example/proxyvm"
	"github.com/ava-labs/avalanchego/vms/example/xsvm/cmd/issue/transfer"
	"github.com/ava-labs/avalanchego/vms/example/xsvm/genesis"
	// "github.com/ava-labs/avalanchego/vms/rpcchainvm/runtime"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	"github.com/ava-labs/avalanchego/wallet/chain/p"
)

var _ = e2e.DescribePChain("[ProxyVM]", func() {
	require := require.New(ginkgo.GinkgoT())

	proxyvmFilename := "rXJsEqRDAFw7svqnNME1rJgi1B5fSVJ28mvgJAUJg7nDv8WYo"

	ginkgo.It("should enable the use of an unmanaged VM process", func() {
		subnetName := "e2e-proxyvm"

		requiredPlugins := map[string]string{
			"xsvm":    "v3m4wPxaHpvGr8qfMeyK6PRW3idZrPHmYcMTt7oXdK47yurVH",
			"proxyvm": proxyvmFilename,
		}
		for pluginName, pluginFilename := range requiredPlugins {
			pluginPath := filepath.Join(e2e.Env.PluginDir, pluginFilename)
			ginkgo.By(fmt.Sprintf("checking that %q plugin binary exists at path %s", pluginName, pluginPath), func() {
				_, err := os.Stat(pluginPath)
				require.NoError(err)
			})
		}

		ginkgo.By(fmt.Sprintf("checking if the %q subnet already exists", subnetName))
		network := e2e.Env.GetNetwork()
		subnets, err := network.GetSubnets()
		require.NoError(err)
		var (
			testSubnet *testnet.Subnet
			pWallet    p.Wallet
		)
		for _, subnet := range subnets {
			if subnet.Spec.Name == subnetName {
				testSubnet = subnet
			}
		}
		keyFactory := secp256k1.Factory{}
		if testSubnet != nil {
			tests.Outf(" subnet exists and will be reused\n")
			if len(os.Getenv("E2E_RESTART_SUBNETS")) > 0 {
				ginkgo.By("restarting subnets")
				require.NoError(network.RestartSubnets(e2e.DefaultContext(), ginkgo.GinkgoWriter, testSubnet))
			}

			ginkgo.By("initializing P-Chain wallet compatible with the subnet")
			nodeURI := e2e.Env.GetRandomNodeURI()
			keychain := secp256k1fx.NewKeychain(testSubnet.Spec.OwningKey)
			baseWallet := e2e.Env.NewWallet(keychain, nodeURI, testSubnet.ID)
			pWallet = baseWallet.P()

		} else {
			tests.Outf(" subnet does not yet exist and will be created\n")

			ginkgo.By("creating a wallet with a funded key that will create the subnet")
			nodeURI := e2e.Env.GetRandomNodeURI()
			keychain := e2e.Env.NewKeychain(1)
			privateKey := keychain.Keys[0]
			baseWallet := e2e.Env.NewWallet(keychain, nodeURI)
			pWallet = baseWallet.P()

			ginkgo.By("defining the specification of subnet to run custom VM's on")
			subnetSpec := testnet.SubnetSpec{
				Name:      subnetName,
				OwningKey: privateKey,
				NodeSpecs: []testnet.NodeSpec{
					{
						Flags: testnet.FlagsMap{
							config.PluginDirKey: e2e.Env.PluginDir,
						},
						Count: 1,
					},
				},
			}
			ginkgo.By("creating 1 xsvm subnets")
			nodeCleanupFunc := e2e.RegisterNodeforCleanup
			if e2e.Env.RetainSubnets {
				tests.Outf(" subnet nodes will be left running\n")
				nodeCleanupFunc = nil
			}
			subnets, err := testnet.CreateSubnets(
				ginkgo.GinkgoWriter,
				e2e.DefaultTimeout,
				pWallet,
				network,
				nodeCleanupFunc,
				subnetSpec,
			)
			require.NoError(err)

			testSubnet = subnets[0]
		}

		ginkgo.By("defining the spec for a chain that will run proxyvm+xsvm")
		fundedKey, err := keyFactory.NewPrivateKey()
		require.NoError(err)
		genesisBytes, err := genesis.Codec.Marshal(genesis.Version, &genesis.Genesis{
			Timestamp: 0,
			Allocations: []genesis.Allocation{
				{
					Address: fundedKey.Address(),
					Balance: math.MaxUint64,
				},
			},
		})
		require.NoError(err)
		blockchainSpec := testnet.BlockchainSpec{
			// Name is proxyvm, but genesisbytes are for
			// xsvm. This is because the genesis bytes
			// will be passed through to xsvm by proxyvm.
			VMName:  "proxyvm",
			Genesis: genesisBytes,
		}

		ginkgo.By("creating chain")
		chainID, err := testnet.CreateBlockchain(
			e2e.DefaultContext(),
			ginkgo.GinkgoWriter,
			pWallet,
			testSubnet.ID,
			blockchainSpec,
			e2e.DefaultTimeout,
		)
		require.NoError(err)

		ginkgo.By("check that proxyvm wrote its address in the expected location")
		nodes, err := testSubnet.GetNodes(network)
		require.NoError(err)
		testNode := nodes[0]
		dataDir, err := testNode.GetConfig().Flags.GetStringVal(config.DataDirKey)
		require.NoError(err)
		vmInitPath := filepath.Join(dataDir, "vm_init", fmt.Sprintf("%s.json", proxyvmFilename))
		_, err = os.Stat(vmInitPath)
		require.NoError(err)

		ginkgo.By("reading the init config written by proxyvm")
		bytes, err := os.ReadFile(vmInitPath)
		require.NoError(err)
		initConfig := proxyvm.InitConfig{}
		err = json.Unmarshal(bytes, &initConfig)
		require.NoError(err)

		ginkgo.By("launching xsvm configured to talk to proxyvm")
		// TODO(marun)
		// - need to start xsvm in a subprocess configured with

		// Now try to perform a balance transfer to test if xsvm is
		// working correctly behind proxyvm

		ginkgo.By(fmt.Sprintf("issuing a transaction on chain %s on subnet %s", chainID, testSubnet.ID))
		nodeURIs, err := testSubnet.GetNodeURIs(network)
		require.NoError(err)
		nodeURI := nodeURIs[0]
		tests.Outf(" issuing transactions on %s (%s)\n", nodeURI.NodeID, nodeURI.URI)
		recipientKey, err := keyFactory.NewPrivateKey()
		require.NoError(err)
		transferTxStatus, err := transfer.Transfer(
			e2e.DefaultContext(),
			&transfer.Config{
				URI:        nodeURI.URI,
				ChainID:    *chainID,
				AssetID:    *chainID,
				Amount:     units.Schmeckle,
				To:         recipientKey.Address(),
				PrivateKey: testSubnet.Spec.FundedKeys[0],
			},
		)
		require.NoError(err)
		tests.Outf(" issued transaction with ID: %s\n", transferTxStatus.TxID)

	})
})
