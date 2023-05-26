package simulation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/ava-labs/avalanchego/chains/atomic"
	"github.com/ava-labs/avalanchego/database/manager"
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/snow"
	"github.com/ava-labs/avalanchego/snow/engine/common"
	"github.com/ava-labs/avalanchego/snow/engine/snowman/block"
	"github.com/ava-labs/avalanchego/utils"
	"github.com/ava-labs/avalanchego/utils/cb58"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/utils/logging"
	"github.com/ava-labs/avalanchego/utils/set"
	"github.com/ava-labs/avalanchego/vms"
	coreth "github.com/ava-labs/coreth/plugin/evm"

	"github.com/ava-labs/coreth/core"
)

// function is copypasta from coreth
func BuildGenesisTest(t *testing.T, genesisJSON string) []byte {
	ss := coreth.StaticService{}

	genesis := &core.Genesis{}
	if err := json.Unmarshal([]byte(genesisJSON), genesis); err != nil {
		t.Fatalf("Problem unmarshaling genesis JSON: %s", err)
	}
	genesisReply, err := ss.BuildGenesis(nil, genesis)
	if err != nil {
		t.Fatalf("Failed to create test genesis")
	}
	genesisBytes, err := formatting.Decode(genesisReply.Encoding, genesisReply.Bytes)
	if err != nil {
		t.Fatalf("Failed to decode genesis bytes: %s", err)
	}
	return genesisBytes
}

func TestEVMGossip(t *testing.T) {
	r := require.New(t)

	var b []byte
	factory := secp256k1.Factory{}

	keys := make([]*secp256k1.PrivateKey, 0, 1)
	testEthAddrs := make([]gethCommon.Address, 0, 1) // testEthAddrs[i] corresponds to testKeys[i]

	for _, key := range []string{
		"24jUJ9vZexUM6expyMcT48LBx27k1m7xpraoV62oSQAHdziao5",
	} {
		b, _ = cb58.Decode(key)
		pk, _ := factory.ToPrivateKey(b)
		keys = append(keys, pk)
		testEthAddrs = append(testEthAddrs, coreth.GetEthAddress(pk))
	}

	evmFactory := &coreth.Factory{}
	nodeConfig := NodeConfig{
		MessageTimeout: 5 * time.Second,
	}
	senderConfig := SenderConfig{
		AppGossipSize: 10, // default in avalanchego
	}

	genesisJSONApricotPhase5 := "{\"config\":{\"chainId\":43111,\"homesteadBlock\":0,\"daoForkBlock\":0,\"daoForkSupport\":true,\"eip150Block\":0,\"eip150Hash\":\"0x2086799aeebeae135c246c65021c82b4e15a2c451340993aacfd2751886514f0\",\"eip155Block\":0,\"eip158Block\":0,\"byzantiumBlock\":0,\"constantinopleBlock\":0,\"petersburgBlock\":0,\"istanbulBlock\":0,\"muirGlacierBlock\":0,\"apricotPhase1BlockTimestamp\":0,\"apricotPhase2BlockTimestamp\":0,\"apricotPhase3BlockTimestamp\":0,\"apricotPhase4BlockTimestamp\":0,\"apricotPhase5BlockTimestamp\":0},\"nonce\":\"0x0\",\"timestamp\":\"0x0\",\"extraData\":\"0x00\",\"gasLimit\":\"0x5f5e100\",\"difficulty\":\"0x0\",\"mixHash\":\"0x0000000000000000000000000000000000000000000000000000000000000000\",\"coinbase\":\"0x0000000000000000000000000000000000000000\",\"alloc\":{\"0100000000000000000000000000000000000000\":{\"code\":\"0x7300000000000000000000000000000000000000003014608060405260043610603d5760003560e01c80631e010439146042578063b6510bb314606e575b600080fd5b605c60048036036020811015605657600080fd5b503560b1565b60408051918252519081900360200190f35b818015607957600080fd5b5060af60048036036080811015608e57600080fd5b506001600160a01b03813516906020810135906040810135906060013560b6565b005b30cd90565b836001600160a01b031681836108fc8690811502906040516000604051808303818888878c8acf9550505050505015801560f4573d6000803e3d6000fd5b505050505056fea26469706673582212201eebce970fe3f5cb96bf8ac6ba5f5c133fc2908ae3dcd51082cfee8f583429d064736f6c634300060a0033\",\"balance\":\"0x0\"}},\"number\":\"0x0\",\"gasUsed\":\"0x0\",\"parentHash\":\"0x0000000000000000000000000000000000000000000000000000000000000000\"}"
	genesisBytes := BuildGenesisTest(t, genesisJSONApricotPhase5)

	issueTime := &time.Time{}

	wg := &sync.WaitGroup{}
	testVMFactory := vmFactory{
		wg:        wg,
		issueTime: issueTime,
		vmFactory: evmFactory,
	}

	config := coreth.Config{}
	config.SetDefaults()
	config.LogLevel = "error"

	configBytes, err := json.Marshal(config)
	r.NoError(err)

	n := 1000
	s, err := New(context.TODO(), testVMFactory, n, nodeConfig, senderConfig, genesisBytes, configBytes)
	r.NoError(err)
	s.Start()

	node := s.Get(0)

	vm := node.GetVM()
	testVM, ok := vm.(*testVM)
	r.True(ok)

	evm, ok := testVM.ChainVM.(*coreth.VM)
	r.True(ok)

	// the node initiating the tx doesn't need to get the gossip message
	wg.Add(n)

	tx := &coreth.Tx{
		UnsignedAtomicTx: &coreth.TestUnsignedTx{
			GasUsedV:                    0,
			AcceptRequestsBlockchainIDV: ids.ID{},
			AcceptRequestsV: &atomic.Requests{
				RemoveRequests: [][]byte{
					utils.RandomBytes(32),
				},
			},
			VerifyV:           nil,
			IDV:               ids.ID{},
			BurnedV:           0,
			UnsignedBytesV:    nil,
			SignedBytesV:      nil,
			InputUTXOsV:       set.Set[ids.ID]{},
			SemanticVerifyV:   nil,
			EVMStateTransferV: nil,
		},
	}
	r.NoError(tx.Sign(coreth.Codec, [][]*secp256k1.PrivateKey{
		{keys[0]},
	}))

	*issueTime = time.Now()
	fmt.Println(0) // first node learns immediately
	r.NoError(evm.IssueTx(tx, true))

	wg.Wait()

}

var _ vms.Factory = (*vmFactory)(nil)

type vmFactory struct {
	wg        *sync.WaitGroup
	vmFactory vms.Factory
	issueTime *time.Time
}

func (v vmFactory) New(logger logging.Logger) (interface{}, error) {
	vm, err := v.vmFactory.New(logger)
	if err != nil {
		return nil, err
	}

	chainVM, ok := vm.(block.ChainVM)
	if !ok {
		return nil, errors.New("unexpected vm")
	}

	return &testVM{
		wg:        v.wg,
		log:       logger,
		ChainVM:   chainVM,
		issueTime: v.issueTime,
	}, nil
}

var _ block.ChainVM = (*testVM)(nil)

type testVM struct {
	wg  *sync.WaitGroup
	log logging.Logger
	block.ChainVM

	nodeID    ids.NodeID
	issueTime *time.Time
	received  bool
	lock      sync.Mutex
}

func (t *testVM) Initialize(
	ctx context.Context,
	chainCtx *snow.Context,
	dbManager manager.Manager,
	genesisBytes []byte,
	upgradeBytes []byte,
	configBytes []byte,
	toEngine chan<- common.Message,
	fxs []*common.Fx,
	appSender common.AppSender,
) error {
	t.nodeID = chainCtx.NodeID
	return t.ChainVM.Initialize(
		ctx,
		chainCtx,
		dbManager,
		genesisBytes,
		upgradeBytes,
		configBytes,
		toEngine,
		fxs,
		appSender,
	)
}

func (t *testVM) AppGossip(ctx context.Context, nodeID ids.NodeID, msg []byte) error {
	t.lock.Lock()
	defer t.lock.Unlock()

	if !t.received {
		defer t.wg.Done()

		t.received = true
		fmt.Println(time.Now().Sub(*t.issueTime).Nanoseconds())
	}

	return t.ChainVM.AppGossip(ctx, nodeID, msg)
}
