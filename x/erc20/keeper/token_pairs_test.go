package keeper_test

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"golang.org/x/exp/slices"

	testconstants "github.com/cosmos/evm/testutil/constants"
	utiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/erc20/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (suite *KeeperTestSuite) TestGetTokenPairs() {
	var (
		ctx    sdk.Context
		expRes []types.TokenPair
	)

	testCases := []struct {
		name     string
		malleate func()
	}{
		{
			"no pair registered", func() { expRes = testconstants.ExampleTokenPairs },
		},
		{
			"1 pair registered",
			func() {
				pair := types.NewTokenPair(utiltx.GenerateAddress(), "coin", types.OWNER_MODULE)
				suite.network.App.Erc20Keeper.SetTokenPair(ctx, pair)
				expRes = testconstants.ExampleTokenPairs
				expRes = append(expRes, pair)
			},
		},
		{
			"2 pairs registered",
			func() {
				pair := types.NewTokenPair(utiltx.GenerateAddress(), "coin", types.OWNER_MODULE)
				pair2 := types.NewTokenPair(utiltx.GenerateAddress(), "coin2", types.OWNER_MODULE)
				suite.network.App.Erc20Keeper.SetTokenPair(ctx, pair)
				suite.network.App.Erc20Keeper.SetTokenPair(ctx, pair2)
				expRes = testconstants.ExampleTokenPairs
				expRes = append(expRes, []types.TokenPair{pair, pair2}...)
			},
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest() // reset
			ctx = suite.network.GetContext()

			tc.malleate()
			res := suite.network.App.Erc20Keeper.GetTokenPairs(ctx)

			suite.Require().ElementsMatch(expRes, res, tc.name)
		})
	}
}

func (suite *KeeperTestSuite) TestGetTokenPairID() {
	baseDenom, err := sdk.GetBaseDenom()
	suite.Require().NoError(err, "failed to get base denom")

	pair := types.NewTokenPair(utiltx.GenerateAddress(), baseDenom, types.OWNER_MODULE)

	testCases := []struct {
		name  string
		token string
		expID []byte
	}{
		{"nil token", "", nil},
		{"valid hex token", utiltx.GenerateAddress().Hex(), []byte{}},
		{"valid hex token", utiltx.GenerateAddress().String(), []byte{}},
	}
	for _, tc := range testCases {
		suite.SetupTest()
		ctx := suite.network.GetContext()

		suite.network.App.Erc20Keeper.SetTokenPair(ctx, pair)

		id := suite.network.App.Erc20Keeper.GetTokenPairID(ctx, tc.token)
		if id != nil {
			suite.Require().Equal(tc.expID, id, tc.name)
		} else {
			suite.Require().Nil(id)
		}
	}
}

func (suite *KeeperTestSuite) TestGetTokenPair() {
	baseDenom, err := sdk.GetBaseDenom()
	suite.Require().NoError(err, "failed to get base denom")

	pair := types.NewTokenPair(utiltx.GenerateAddress(), baseDenom, types.OWNER_MODULE)

	testCases := []struct {
		name string
		id   []byte
		ok   bool
	}{
		{"nil id", nil, false},
		{"valid id", pair.GetID(), true},
		{"pair not found", []byte{}, false},
	}
	for _, tc := range testCases {
		suite.SetupTest()
		ctx := suite.network.GetContext()

		suite.network.App.Erc20Keeper.SetTokenPair(ctx, pair)
		p, found := suite.network.App.Erc20Keeper.GetTokenPair(ctx, tc.id)
		if tc.ok {
			suite.Require().True(found, tc.name)
			suite.Require().Equal(pair, p, tc.name)
		} else {
			suite.Require().False(found, tc.name)
		}
	}
}

func (suite *KeeperTestSuite) TestDeleteTokenPair() {
	baseDenom, err := sdk.GetBaseDenom()
	suite.Require().NoError(err, "failed to get base denom")

	var ctx sdk.Context
	pair := types.NewTokenPair(utiltx.GenerateAddress(), baseDenom, types.OWNER_MODULE)
	id := pair.GetID()

	testCases := []struct {
		name     string
		id       []byte
		malleate func()
		ok       bool
	}{
		{"nil id", nil, func() {}, false},
		{"pair not found", []byte{}, func() {}, false},
		{"valid id", id, func() {}, true},
		{
			"delete tokenpair",
			id,
			func() {
				suite.network.App.Erc20Keeper.DeleteTokenPair(ctx, pair)
			},
			false,
		},
	}
	for _, tc := range testCases {
		suite.SetupTest()
		ctx = suite.network.GetContext()
		suite.network.App.Erc20Keeper.SetToken(ctx, pair)

		tc.malleate()
		p, found := suite.network.App.Erc20Keeper.GetTokenPair(ctx, tc.id)
		if tc.ok {
			suite.Require().True(found, tc.name)
			suite.Require().Equal(pair, p, tc.name)
		} else {
			suite.Require().False(found, tc.name)
		}
	}
}

func (suite *KeeperTestSuite) TestIsTokenPairRegistered() {
	baseDenom, err := sdk.GetBaseDenom()
	suite.Require().NoError(err, "failed to get base denom")

	var ctx sdk.Context
	pair := types.NewTokenPair(utiltx.GenerateAddress(), baseDenom, types.OWNER_MODULE)

	testCases := []struct {
		name string
		id   []byte
		ok   bool
	}{
		{"valid id", pair.GetID(), true},
		{"pair not found", []byte{}, false},
	}
	for _, tc := range testCases {
		suite.SetupTest()
		ctx = suite.network.GetContext()

		suite.network.App.Erc20Keeper.SetTokenPair(ctx, pair)
		found := suite.network.App.Erc20Keeper.IsTokenPairRegistered(ctx, tc.id)
		if tc.ok {
			suite.Require().True(found, tc.name)
		} else {
			suite.Require().False(found, tc.name)
		}
	}
}

func (suite *KeeperTestSuite) TestIsERC20Registered() {
	var ctx sdk.Context
	addr := utiltx.GenerateAddress()
	pair := types.NewTokenPair(addr, "coin", types.OWNER_MODULE)

	testCases := []struct {
		name     string
		erc20    common.Address
		malleate func()
		ok       bool
	}{
		{"nil erc20 address", common.Address{}, func() {}, false},
		{"valid erc20 address", pair.GetERC20Contract(), func() {}, true},
		{
			"deleted erc20 map",
			pair.GetERC20Contract(),
			func() {
				suite.network.App.Erc20Keeper.DeleteTokenPair(ctx, pair)
			},
			false,
		},
	}
	for _, tc := range testCases {
		suite.SetupTest()
		ctx := suite.network.GetContext()

		suite.network.App.Erc20Keeper.SetToken(ctx, pair)

		tc.malleate()

		found := suite.network.App.Erc20Keeper.IsERC20Registered(ctx, tc.erc20)

		if tc.ok {
			suite.Require().True(found, tc.name)
		} else {
			suite.Require().False(found, tc.name)
		}
	}
}

func (suite *KeeperTestSuite) TestIsDenomRegistered() {
	var ctx sdk.Context
	addr := utiltx.GenerateAddress()
	pair := types.NewTokenPair(addr, "coin", types.OWNER_MODULE)

	testCases := []struct {
		name     string
		denom    string
		malleate func()
		ok       bool
	}{
		{"empty denom", "", func() {}, false},
		{"valid denom", pair.GetDenom(), func() {}, true},
		{
			"deleted denom map",
			pair.GetDenom(),
			func() {
				suite.network.App.Erc20Keeper.DeleteTokenPair(ctx, pair)
			},
			false,
		},
	}
	for _, tc := range testCases {
		suite.SetupTest()
		ctx = suite.network.GetContext()

		suite.network.App.Erc20Keeper.SetToken(ctx, pair)

		tc.malleate()

		found := suite.network.App.Erc20Keeper.IsDenomRegistered(ctx, tc.denom)

		if tc.ok {
			suite.Require().True(found, tc.name)
		} else {
			suite.Require().False(found, tc.name)
		}
	}
}

func (suite *KeeperTestSuite) TestGetTokenDenom() {
	var ctx sdk.Context
	tokenAddress := utiltx.GenerateAddress()
	tokenDenom := "token"

	testCases := []struct {
		name        string
		tokenDenom  string
		malleate    func()
		expError    bool
		errContains string
	}{
		{
			"denom found",
			tokenDenom,
			func() {
				pair := types.NewTokenPair(tokenAddress, tokenDenom, types.OWNER_MODULE)
				suite.network.App.Erc20Keeper.SetTokenPair(ctx, pair)
				suite.network.App.Erc20Keeper.SetERC20Map(ctx, tokenAddress, pair.GetID())
			},
			true,
			"",
		},
		{
			"denom not found",
			tokenDenom,
			func() {
				address := utiltx.GenerateAddress()
				pair := types.NewTokenPair(address, tokenDenom, types.OWNER_MODULE)
				suite.network.App.Erc20Keeper.SetTokenPair(ctx, pair)
				suite.network.App.Erc20Keeper.SetERC20Map(ctx, address, pair.GetID())
			},
			false,
			fmt.Sprintf("token '%s' not registered", tokenAddress),
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()

			tc.malleate()
			res, err := suite.network.App.Erc20Keeper.GetTokenDenom(ctx, tokenAddress)

			if tc.expError {
				suite.Require().NoError(err)
				suite.Require().Equal(res, tokenDenom)
			} else {
				suite.Require().Error(err, "expected an error while getting the token denom")
				suite.Require().ErrorContains(err, tc.errContains)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestGetNativePrecompiles() {
	var ctx sdk.Context
	testAddr := utiltx.GenerateAddress()
	defaultWEVMOSAddr := common.HexToAddress(testconstants.WEVMOSContractMainnet)

	testCases := []struct {
		name     string
		malleate func()
		expRes   []string
	}{
		{
			"default native precompiles registered",
			func() {},
			[]string{defaultWEVMOSAddr.Hex()},
		},
		{
			"no native precompiles registered",
			func() {
				suite.network.App.Erc20Keeper.DeleteNativePrecompile(ctx, defaultWEVMOSAddr)
			},
			nil,
		},
		{
			"multiple native precompiles available",
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]string{defaultWEVMOSAddr.Hex(), testAddr.Hex()},
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()
			tc.malleate()

			slices.Sort(tc.expRes)
			res := suite.network.App.Erc20Keeper.GetNativePrecompiles(ctx)
			suite.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}

func (suite *KeeperTestSuite) TestSetNativePrecompile() {
	var ctx sdk.Context
	testAddr := utiltx.GenerateAddress()
	defaultWEVMOSAddr := common.HexToAddress(testconstants.WEVMOSContractMainnet)

	testCases := []struct {
		name     string
		addrs    []common.Address
		malleate func()
		expRes   []string
	}{
		{
			"set new native precompile",
			[]common.Address{testAddr},
			func() {},
			[]string{defaultWEVMOSAddr.Hex(), testAddr.Hex()},
		},
		{
			"set duplicate native precompile",
			[]common.Address{testAddr},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]string{defaultWEVMOSAddr.Hex(), testAddr.Hex()},
		},
		{
			"set non-eip55 native precompile variations",
			[]common.Address{
				common.HexToAddress(strings.ToLower(testAddr.Hex())),
				common.HexToAddress(strings.ToUpper(testAddr.Hex())),
			},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]string{defaultWEVMOSAddr.Hex(), testAddr.Hex()},
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()
			tc.malleate()

			slices.Sort(tc.expRes)
			for _, addr := range tc.addrs {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, addr)
			}
			res := suite.network.App.Erc20Keeper.GetNativePrecompiles(ctx)
			suite.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}

func (suite *KeeperTestSuite) TestDeleteNativePrecompile() {
	var ctx sdk.Context
	testAddr := utiltx.GenerateAddress()
	defaultWEVMOSAddr := common.HexToAddress(testconstants.WEVMOSContractMainnet)
	unavailableAddr := common.HexToAddress("unavailable")

	testCases := []struct {
		name     string
		addrs    []common.Address
		malleate func()
		expRes   []string
	}{
		{
			"delete all native precompiles",
			[]common.Address{defaultWEVMOSAddr, testAddr},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			nil,
		},
		{
			"delete unavailable native precompile",
			[]common.Address{unavailableAddr},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]string{defaultWEVMOSAddr.Hex(), testAddr.Hex()},
		},
		{
			"delete default native precompile",
			[]common.Address{defaultWEVMOSAddr},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
		{
			"delete new native precompile",
			[]common.Address{testAddr},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]string{defaultWEVMOSAddr.Hex()},
		},
		{
			"delete with non-eip55 native precompile lower variation",
			[]common.Address{
				common.HexToAddress(strings.ToLower(defaultWEVMOSAddr.Hex())),
			},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
		{
			"delete with non-eip55 native precompile upper variation",
			[]common.Address{
				common.HexToAddress(strings.ToUpper(defaultWEVMOSAddr.Hex())),
			},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
		{
			"delete multiple of same native precompile",
			[]common.Address{
				defaultWEVMOSAddr,
				defaultWEVMOSAddr,
				defaultWEVMOSAddr,
			},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()
			tc.malleate()

			slices.Sort(tc.expRes)
			for _, addr := range tc.addrs {
				suite.network.App.Erc20Keeper.DeleteNativePrecompile(ctx, addr)
			}
			res := suite.network.App.Erc20Keeper.GetNativePrecompiles(ctx)
			suite.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}

func (suite *KeeperTestSuite) TestIsNativePrecompileAvailable() {
	var ctx sdk.Context
	testAddr := utiltx.GenerateAddress()
	defaultWEVMOSAddr := common.HexToAddress(testconstants.WEVMOSContractMainnet)
	unavailableAddr := common.HexToAddress("unavailable")

	testCases := []struct {
		name     string
		addrs    []common.Address
		malleate func()
		expRes   []bool
	}{
		{
			"all native precompiles are available",
			[]common.Address{defaultWEVMOSAddr, testAddr},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]bool{true, true},
		},
		{
			"only default native precompile is available",
			[]common.Address{defaultWEVMOSAddr, testAddr},
			func() {},
			[]bool{true, false},
		},
		{
			"unavailable native precompile is unavailable",
			[]common.Address{unavailableAddr},
			func() {},
			[]bool{false},
		},
		{
			"non-eip55 native precompiles are available",
			[]common.Address{
				testAddr,
				common.HexToAddress(strings.ToLower(testAddr.Hex())),
				common.HexToAddress(strings.ToUpper(testAddr.Hex())),
			},
			func() {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, testAddr)
			},
			[]bool{true, true, true},
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()
			tc.malleate()

			for _, addr := range tc.addrs {
				suite.network.App.Erc20Keeper.SetNativePrecompile(ctx, addr)
			}

			res := make([]bool, 0, len(tc.expRes))
			for _, addr := range tc.addrs {
				res = append(res, suite.network.App.Erc20Keeper.IsNativePrecompileAvailable(ctx, addr))
			}

			suite.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}

func (suite *KeeperTestSuite) TestGetDynamicPrecompiles() {
	var ctx sdk.Context
	testAddr := utiltx.GenerateAddress()

	testCases := []struct {
		name     string
		malleate func()
		expRes   []string
	}{
		{
			"no dynamic precompiles registered",
			func() {},
			nil,
		},
		{
			"dynamic precompile available",
			func() {
				suite.network.App.Erc20Keeper.SetDynamicPrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()
			tc.malleate()

			slices.Sort(tc.expRes)
			res := suite.network.App.Erc20Keeper.GetDynamicPrecompiles(ctx)
			suite.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}
func (suite *KeeperTestSuite) TestSetDynamicPrecompile() {
	var ctx sdk.Context
	testAddr := utiltx.GenerateAddress()

	testCases := []struct {
		name     string
		addrs    []common.Address
		malleate func()
		expRes   []string
	}{
		{
			"set new dynamic precompile",
			[]common.Address{testAddr},
			func() {},
			[]string{testAddr.Hex()},
		},
		{
			"set duplicate dynamic precompile",
			[]common.Address{testAddr},
			func() {
				suite.network.App.Erc20Keeper.SetDynamicPrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
		{
			"set non-eip55 dynamic precompile variations",
			[]common.Address{
				common.HexToAddress(strings.ToLower(testAddr.Hex())),
				common.HexToAddress(strings.ToUpper(testAddr.Hex())),
			},
			func() {
				suite.network.App.Erc20Keeper.SetDynamicPrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()
			tc.malleate()

			slices.Sort(tc.expRes)
			for _, addr := range tc.addrs {
				suite.network.App.Erc20Keeper.SetDynamicPrecompile(ctx, addr)
			}
			res := suite.network.App.Erc20Keeper.GetDynamicPrecompiles(ctx)
			suite.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}

func (suite *KeeperTestSuite) TestDeleteDynamicPrecompile() {
	var ctx sdk.Context
	testAddr := utiltx.GenerateAddress()
	unavailableAddr := common.HexToAddress("unavailable")

	testCases := []struct {
		name     string
		addrs    []common.Address
		malleate func()
		expRes   []string
	}{
		{
			"delete new dynamic precompiles",
			[]common.Address{testAddr},
			func() {
				suite.network.App.Erc20Keeper.SetDynamicPrecompile(ctx, testAddr)
			},
			nil,
		},
		{
			"delete unavailable dynamic precompile",
			[]common.Address{unavailableAddr},
			func() {
				suite.network.App.Erc20Keeper.SetDynamicPrecompile(ctx, testAddr)
			},
			[]string{testAddr.Hex()},
		},
		{
			"delete with non-eip55 dynamic precompile lower variation",
			[]common.Address{
				common.HexToAddress(strings.ToLower(testAddr.Hex())),
			},
			func() {
				suite.network.App.Erc20Keeper.SetDynamicPrecompile(ctx, testAddr)
			},
			nil,
		},
		{
			"delete with non-eip55 dynamic precompile upper variation",
			[]common.Address{
				common.HexToAddress(strings.ToUpper(testAddr.Hex())),
			},
			func() {
				suite.network.App.Erc20Keeper.SetDynamicPrecompile(ctx, testAddr)
			},
			nil,
		},
		{
			"delete multiple of same dynamic precompile",
			[]common.Address{
				testAddr,
				testAddr,
				testAddr,
			},
			func() {
				suite.network.App.Erc20Keeper.SetDynamicPrecompile(ctx, testAddr)
			},
			nil,
		},
	}
	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()
			tc.malleate()

			slices.Sort(tc.expRes)
			for _, addr := range tc.addrs {
				suite.network.App.Erc20Keeper.DeleteDynamicPrecompile(ctx, addr)
			}
			res := suite.network.App.Erc20Keeper.GetDynamicPrecompiles(ctx)
			suite.Require().ElementsMatch(res, tc.expRes, tc.name)
		})
	}
}

func (suite *KeeperTestSuite) TestIsDynamicPrecompileAvailable() {}
