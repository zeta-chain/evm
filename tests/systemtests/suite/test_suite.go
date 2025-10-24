package suite

import (
	"math/big"
	"testing"
	"time"

	"github.com/cosmos/evm/tests/systemtests/clients"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/systemtests"
)

// SystemTestSuite implements the TestSuite interface and
// provides methods for managing test lifecycle,
// sending transactions, querying state,
// and managing expected mempool state.
type SystemTestSuite struct {
	*systemtests.SystemUnderTest
	options *TestOptions

	// Clients
	EthClient    *clients.EthClient
	CosmosClient *clients.CosmosClient

	// Most recently retrieved base fee
	baseFee *big.Int

	// Expected transaction hashes
	expPendingTxs []*TxInfo
	expQueuedTxs  []*TxInfo
}

func NewSystemTestSuite(t *testing.T) *SystemTestSuite {
	ethClient, err := clients.NewEthClient()
	require.NoError(t, err)

	cosmosClient, err := clients.NewCosmosClient()
	require.NoError(t, err)

	return &SystemTestSuite{
		SystemUnderTest: systemtests.Sut,
		EthClient:       ethClient,
		CosmosClient:    cosmosClient,
	}
}

// SetupTest initializes the test suite by resetting and starting the chain, then awaiting 2 blocks
func (s *SystemTestSuite) SetupTest(t *testing.T, nodeStartArgs ...string) {
	if len(nodeStartArgs) == 0 {
		nodeStartArgs = DefaultNodeArgs()
	}

	s.ResetChain(t)
	s.StartChain(t, nodeStartArgs...)
	s.AwaitNBlocks(t, 2)
}

// BeforeEach resets the expected mempool state and retrieves the current base fee before each test case
func (s *SystemTestSuite) BeforeEachCase(t *testing.T) {
	// Reset expected pending/queued transactions
	s.expPendingTxs = []*TxInfo{}
	s.expQueuedTxs = []*TxInfo{}

	// Get current base fee
	currentBaseFee, err := s.GetLatestBaseFee("node0")
	require.NoError(t, err)

	s.baseFee = currentBaseFee
}

// JustAfterEach checks the expected mempool state right after each test case
func (s *SystemTestSuite) AfterEachAction(t *testing.T) {
	// Check pending txs exist in mempool or already committed - concurrently
	err := s.CheckTxsPendingAsync(s.GetExpPendingTxs())
	require.NoError(t, err)

	// Check queued txs only exist in local mempool (queued txs should be only EVM txs)
	err = s.CheckTxsQueuedSync(s.GetExpQueuedTxs())
	require.NoError(t, err)

	// Wait for block commit
	s.AwaitNBlocks(t, 1)

	// Get current base fee and set it to suite.baseFee
	currentBaseFee, err := s.GetLatestBaseFee("node0")
	require.NoError(t, err)

	s.baseFee = currentBaseFee
}

// AfterEach waits for all expected pending transactions to be committed
func (s *SystemTestSuite) AfterEachCase(t *testing.T) {
	// Check all expected pending txs are committed
	for _, txInfo := range s.GetExpPendingTxs() {
		err := s.WaitForCommit(txInfo.DstNodeID, txInfo.TxHash, txInfo.TxType, time.Second*60)
		require.NoError(t, err)
	}

	// Check all evm pending txs are cleared in mempool
	for i := range s.Nodes() {
		pending, _, err := s.TxPoolContent(s.Node(i), TxTypeEVM)
		require.NoError(t, err)

		require.Len(t, pending, 0, "pending txs are not cleared in mempool")
	}

	// Check all cosmos pending txs are cleared in mempool
	for i := range s.Nodes() {
		pending, _, err := s.TxPoolContent(s.Node(i), TxTypeCosmos)
		require.NoError(t, err)

		require.Len(t, pending, 0, "pending txs are not cleared in mempool")
	}

	// Wait for block commit
	s.AwaitNBlocks(t, 1)
}
