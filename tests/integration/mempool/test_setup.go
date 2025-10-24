package mempool

import (
	"time"

	"github.com/stretchr/testify/suite"

	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/keyring"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

// MempoolIntegrationTestSuite is the base test suite for mempool integration tests.
// It provides the infrastructure to test mempool behavior without mocks.
type IntegrationTestSuite struct {
	suite.Suite

	create  network.CreateEvmApp
	options []network.ConfigOption
	network *network.UnitTestNetwork
	factory factory.TxFactory
	keyring keyring.Keyring
}

// NewMempoolIntegrationTestSuite creates a new instance of the test suite.
func NewMempoolIntegrationTestSuite(create network.CreateEvmApp, options ...network.ConfigOption) *IntegrationTestSuite {
	return &IntegrationTestSuite{
		create:  create,
		options: options,
	}
}

// SetupTest initializes the test environment with default settings.
func (s *IntegrationTestSuite) SetupTest() {
	s.SetupTestWithChainID(testconstants.ExampleChainID)
}

// SetupTestWithChainID initializes the test environment with a specific chain ID.
func (s *IntegrationTestSuite) SetupTestWithChainID(chainID testconstants.ChainID) {
	s.keyring = keyring.New(20)

	options := []network.ConfigOption{
		network.WithChainID(chainID),
		network.WithPreFundedAccounts(s.keyring.GetAllAccAddrs()...),
	}
	options = append(options, s.options...)

	nw := network.NewUnitTestNetwork(s.create, options...)
	gh := grpc.NewIntegrationHandler(nw)
	tf := factory.New(nw, gh)

	// Advance to block 2+ where mempool is designed to operate
	// This ensures proper headers, StateDB, and fee market initialization
	err := nw.NextBlock()
	s.Require().NoError(err)
	err = nw.NextBlock()
	s.Require().NoError(err)

	// Wait for mempool async reset goroutines to complete
	// NextBlock() triggers chain head events that start async goroutines to reset
	// the mempool state. Without this wait, tests can start before the reset completes,
	// causing race conditions with stale mempool state.
	time.Sleep(100 * time.Millisecond)

	// Ensure mempool is in ready state by verifying block height
	s.Require().Equal(int64(3), nw.GetContext().BlockHeight())

	// Verify mempool is accessible and operational
	mempool := nw.App.GetMempool()
	s.Require().NotNil(mempool, "mempool should be accessible")

	// Verify initial mempool state
	initialCount := mempool.CountTx()
	s.Require().Equal(0, initialCount, "mempool should be empty initially")

	s.network = nw
	s.factory = tf
}

// FundAccount funds an account with a specific amount of a given denomination.
func (s *IntegrationTestSuite) FundAccount(addr sdk.AccAddress, amount sdkmath.Int, denom string) {
	coins := sdk.NewCoins(sdk.NewCoin(denom, amount))

	// Use the bank keeper to mint and send coins to the account
	err := s.network.App.GetBankKeeper().MintCoins(s.network.GetContext(), minttypes.ModuleName, coins)
	s.Require().NoError(err)

	err = s.network.App.GetBankKeeper().SendCoinsFromModuleToAccount(s.network.GetContext(), minttypes.ModuleName, addr, coins)
	s.Require().NoError(err)
}

// GetAllBalances returns all balances for the given account address.
func (s *IntegrationTestSuite) GetAllBalances(addr sdk.AccAddress) sdk.Coins {
	return s.network.App.GetBankKeeper().GetAllBalances(s.network.GetContext(), addr)
}

// TestBasicSetupAndReadiness tests comprehensive mempool initialization and readiness
func (s *IntegrationTestSuite) TestBasicSetupAndReadiness() {
	testCases := []struct {
		name     string
		testFunc func()
	}{
		{
			name: "Infrastructure is properly initialized",
			testFunc: func() {
				s.Require().NotNil(s.network, "network should be initialized")
				s.Require().NotNil(s.keyring, "keyring should be initialized")
				s.Require().NotNil(s.factory, "factory should be initialized")
			},
		},
		{
			name: "Keys are properly generated and accessible",
			testFunc: func() {
				key0 := s.keyring.GetKey(0)
				key1 := s.keyring.GetKey(1)
				key2 := s.keyring.GetKey(2)
				s.Require().NotNil(key0, "key 0 should exist")
				s.Require().NotNil(key1, "key 1 should exist")
				s.Require().NotNil(key2, "key 2 should exist")

				// Verify keys have different addresses
				s.Require().NotEqual(key0.AccAddr.String(), key1.AccAddr.String(), "keys should have different addresses")
				s.Require().NotEqual(key0.AccAddr.String(), key2.AccAddr.String(), "keys should have different addresses")
			},
		},
		{
			name: "Block height is at expected level",
			testFunc: func() {
				s.Require().Equal(int64(3), s.network.GetContext().BlockHeight(), "should be at block 3 after setup")
				s.Require().True(s.network.GetContext().BlockHeight() >= 2, "mempool requires block height >= 2")
			},
		},
		{
			name: "Accounts are properly funded",
			testFunc: func() {
				key0 := s.keyring.GetKey(0)
				key1 := s.keyring.GetKey(1)

				bal0 := s.GetAllBalances(key0.AccAddr)
				bal1 := s.GetAllBalances(key1.AccAddr)

				s.Require().False(bal0.IsZero(), "key 0 should have positive balance")
				s.Require().False(bal1.IsZero(), "key 1 should have positive balance")
				s.Require().True(bal0.AmountOf(s.network.GetBaseDenom()).IsPositive(), "should have base denom balance")
			},
		},
		{
			name: "Mempool is in ready operational state",
			testFunc: func() {
				mempool := s.network.App.GetMempool()
				s.Require().NotNil(mempool, "mempool should be accessible")

				// Verify mempool is empty initially
				initialCount := mempool.CountTx()
				s.Require().Equal(0, initialCount, "mempool should be empty initially")

				// Verify mempool accepts block height check (should not panic or error)
				ctx := s.network.GetContext()
				s.Require().True(ctx.BlockHeight() >= 2, "context should be at block 2+ for mempool readiness")
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, tc.testFunc)
	}

	s.T().Logf("All setup validation passed - mempool ready at block %d", s.network.GetContext().BlockHeight())
}
