package types_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	exampleapp "github.com/cosmos/evm/evmd"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/x/erc20/types"
)

type ParamsTestSuite struct {
	suite.Suite
}

func TestParamsTestSuite(t *testing.T) {
	suite.Run(t, new(ParamsTestSuite))
}

func (suite *ParamsTestSuite) TestParamsValidate() {
	testCases := []struct {
		name        string
		malleate    func() types.Params
		expError    bool
		errContains string
	}{
		{
			"default",
			types.DefaultParams,
			false,
			"",
		},
		{
			"valid",
			func() types.Params { return types.NewParams(true, map[string]bool{}, map[string]bool{}, true) },
			false,
			"",
		},
		{
			"valid address - dynamic precompile",
			func() types.Params {
				return types.NewParams(true, map[string]bool{}, map[string]bool{testconstants.WEVMOSContractMainnet: true}, true)
			},
			false,
			"",
		},
		{
			"valid address - native precompile",
			func() types.Params {
				return types.NewParams(true, map[string]bool{testconstants.WEVMOSContractMainnet: true}, map[string]bool{}, true)
			},
			false,
			"",
		},
		{
			"sorted address",
			// order of creation shouldn't matter since it should be sorted when defining new param
			func() types.Params {
				return types.NewParams(true, map[string]bool{testconstants.WEVMOSContractTestnet: true, testconstants.WEVMOSContractMainnet: true}, map[string]bool{}, true)
			},
			false,
			"",
		},
		{
			"unsorted address",
			// order of creation shouldn't matter since it should be sorted when defining new param
			func() types.Params {
				return types.NewParams(true, map[string]bool{testconstants.WEVMOSContractMainnet: true, testconstants.WEVMOSContractTestnet: true}, map[string]bool{}, true)
			},
			false,
			"",
		},
		{
			"empty",
			func() types.Params { return types.Params{} },
			false,
			"",
		},
		{
			"invalid address - native precompile",
			func() types.Params {
				return types.NewParams(true, map[string]bool{"qq": true}, map[string]bool{}, true)
			},
			true,
			"invalid precompile",
		},
		{
			"invalid address - dynamic precompile",
			func() types.Params {
				return types.NewParams(true, map[string]bool{}, map[string]bool{"0xqq": true}, true)
			},
			true,
			"invalid precompile",
		},
		{
			"repeated address in different params",
			func() types.Params {
				return types.NewParams(true, map[string]bool{testconstants.WEVMOSContractMainnet: true}, map[string]bool{testconstants.WEVMOSContractMainnet: true}, true)
			},
			true,
			"duplicate precompile",
		},
		{
			"repeated address - one EIP-55 other not",
			func() types.Params {
				return types.NewParams(true, map[string]bool{}, map[string]bool{"0xcc491f589b45d4a3c679016195b3fb87d7848210": true, "0xcc491f589B45d4a3C679016195B3FB87D7848210": true}, true)
			},
			true,
			"duplicate precompile",
		},
	}

	for _, tc := range testCases {
		p := tc.malleate()
		err := p.Validate()

		if tc.expError {
			suite.Require().Error(err, tc.name)
			suite.Require().ErrorContains(err, tc.errContains)
		} else {
			suite.Require().NoError(err, tc.name)
		}
	}
}

func (suite *ParamsTestSuite) TestIsNativePrecompile() {
	testCases := []struct {
		name     string
		malleate func() types.Params
		addr     common.Address
		expRes   bool
	}{
		{
			"default",
			func() types.Params { return exampleapp.NewErc20GenesisState().Params },
			common.HexToAddress(testconstants.WEVMOSContractMainnet),
			true,
		},
		{
			"not native precompile",
			func() types.Params { return types.NewParams(true, nil, nil, true) },
			common.HexToAddress(testconstants.WEVMOSContractMainnet),
			false,
		},
		{
			"EIP-55 address - is native precompile",
			func() types.Params {
				return types.NewParams(true, map[string]bool{"0xcc491f589B45d4a3C679016195B3FB87D7848210": true}, nil, true)
			},
			common.HexToAddress(testconstants.WEVMOSContractTestnet),
			true,
		},
		{
			"NOT EIP-55 address - is native precompile",
			func() types.Params {
				return types.NewParams(true, map[string]bool{"0xcc491f589b45d4a3c679016195b3fb87d7848210": true}, nil, true)
			},
			common.HexToAddress(testconstants.WEVMOSContractTestnet),
			true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			p := tc.malleate()
			suite.Require().Equal(tc.expRes, p.IsNativePrecompile(tc.addr), tc.name)
		})
	}
}

func (suite *ParamsTestSuite) TestIsDynamicPrecompile() {
	testCases := []struct {
		name     string
		malleate func() types.Params
		addr     common.Address
		expRes   bool
	}{
		{
			"default - not dynamic precompile",
			types.DefaultParams,
			common.HexToAddress(testconstants.WEVMOSContractMainnet),
			false,
		},
		{
			"no dynamic precompiles",
			func() types.Params { return types.NewParams(true, nil, nil, true) },
			common.HexToAddress(testconstants.WEVMOSContractMainnet),
			false,
		},
		{
			"EIP-55 address - is dynamic precompile",
			func() types.Params {
				return types.NewParams(true, nil, map[string]bool{"0xcc491f589B45d4a3C679016195B3FB87D7848210": true}, true)
			},
			common.HexToAddress(testconstants.WEVMOSContractTestnet),
			true,
		},
		{
			"NOT EIP-55 address - is dynamic precompile",
			func() types.Params {
				return types.NewParams(true, nil, map[string]bool{"0xcc491f589b45d4a3c679016195b3fb87d7848210": true}, true)
			},
			common.HexToAddress(testconstants.WEVMOSContractTestnet),
			true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			p := tc.malleate()
			suite.Require().Equal(tc.expRes, p.IsDynamicPrecompile(tc.addr), tc.name)
		})
	}
}

func TestValidatePrecompiles(t *testing.T) {
	testCases := []struct {
		name        string
		precompiles map[string]bool
		expError    bool
		errContains string
	}{
		{
			"invalid precompile address",
			map[string]bool{"0xct491f589b45d4a3c679016195b3fb87d7848210": true, "0xcc491f589B45d4a3C679016195B3FB87D7848210": true},
			true,
			"invalid precompile",
		},
		{
			"same address but one EIP-55 and other don't",
			map[string]bool{"0xcc491f589b45d4a3c679016195b3fb87d7848210": true, "0xcc491f589B45d4a3C679016195B3FB87D7848210": true},
			false,
			"",
		},
	}
	for _, tc := range testCases {
		err := types.ValidatePrecompiles(tc.precompiles)
		if tc.expError {
			require.Error(t, err, tc.name)
			require.ErrorContains(t, err, tc.errContains)
		} else {
			require.NoError(t, err, tc.name)
		}
	}
}
