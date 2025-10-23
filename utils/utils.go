package utils

import (
	"cmp"
	"fmt"
	"math/big"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/pkg/errors"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	ibctransfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/crypto/types/multisig"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

// EthHexToCosmosAddr takes a given Hex string and derives a Cosmos SDK account address
// from it.
func EthHexToCosmosAddr(hexAddr string) sdk.AccAddress {
	return EthToCosmosAddr(common.HexToAddress(hexAddr))
}

// EthToCosmosAddr converts a given Ethereum style address to an SDK address.
func EthToCosmosAddr(addr common.Address) sdk.AccAddress {
	return addr.Bytes()
}

// Bech32ToHexAddr converts a given Bech32 address string and converts it to
// an Ethereum address.
func Bech32ToHexAddr(bech32Addr string) (common.Address, error) {
	accAddr, err := sdk.AccAddressFromBech32(bech32Addr)
	if err != nil {
		return common.Address{}, errorsmod.Wrapf(err, "failed to convert bech32 string to address")
	}

	return CosmosToEthAddr(accAddr), nil
}

// CosmosToEthAddr converts a given SDK account address to
// an Ethereum address.
func CosmosToEthAddr(accAddr sdk.AccAddress) common.Address {
	return common.BytesToAddress(accAddr.Bytes())
}

// Bech32StringFromHexAddress takes a given Hex string and derives a Cosmos SDK account address
// from it.
func Bech32StringFromHexAddress(hexAddr string) string {
	return sdk.AccAddress(common.HexToAddress(hexAddr).Bytes()).String()
}

// HexAddressFromBech32String converts a hex address to a bech32 encoded address.
func HexAddressFromBech32String(addr string) (common.Address, error) {
	decodeFns := []func(string) ([]byte, error){
		func(s string) ([]byte, error) {
			accAddr, err := sdk.AccAddressFromBech32(s)
			if err != nil {
				return nil, err
			}
			return accAddr.Bytes(), nil
		},
		func(s string) ([]byte, error) {
			valAddr, err := sdk.ValAddressFromBech32(s)
			if err != nil {
				return nil, err
			}
			return valAddr.Bytes(), nil
		},
		func(s string) ([]byte, error) {
			consAddr, err := sdk.ConsAddressFromBech32(s)
			if err != nil {
				return nil, err
			}
			return consAddr.Bytes(), nil
		},
	}

	var lastErr error
	for _, fn := range decodeFns {
		bz, err := fn(addr)
		if err == nil {
			return common.BytesToAddress(bz), nil
		}
		lastErr = err
	}
	return common.Address{}, errorsmod.Wrapf(lastErr, "failed to convert bech32 string to address")
}

// IsSupportedKey returns true if the pubkey type is supported by the chain
// (i.e. eth_secp256k1, amino multisig, ed25519).
// NOTE: Nested multisigs are not supported.
func IsSupportedKey(pubkey cryptotypes.PubKey) bool {
	switch pubkey := pubkey.(type) {
	case *ethsecp256k1.PubKey, *ed25519.PubKey:
		return true
	case multisig.PubKey:
		if len(pubkey.GetPubKeys()) == 0 {
			return false
		}

		for _, pk := range pubkey.GetPubKeys() {
			switch pk.(type) {
			case *ethsecp256k1.PubKey, *ed25519.PubKey:
				continue
			default:
				// Nested multisigs are unsupported
				return false
			}
		}

		return true
	default:
		return false
	}
}

// GetAccAddressFromBech32 returns the sdk.Account address of given address,
// while also changing bech32 human readable prefix (HRP) to the value set on
// the global sdk.Config (eg: `evmos`).
//
// The function fails if the provided bech32 address is invalid.
func GetAccAddressFromBech32(address string) (sdk.AccAddress, error) {
	bech32Prefix := strings.SplitN(address, "1", 2)[0]
	if bech32Prefix == address {
		return nil, errorsmod.Wrapf(errortypes.ErrInvalidAddress, "invalid bech32 address: %s", address)
	}

	addressBz, err := sdk.GetFromBech32(address, bech32Prefix)
	if err != nil {
		return nil, errorsmod.Wrapf(errortypes.ErrInvalidAddress, "invalid address %s, %s", address, err.Error())
	}

	// safety check: shouldn't happen
	if err := sdk.VerifyAddressFormat(addressBz); err != nil {
		return nil, err
	}

	return sdk.AccAddress(addressBz), nil
}

// CreateAccAddressFromBech32 creates an AccAddress from a Bech32 string.
func CreateAccAddressFromBech32(address string, bech32prefix string) (addr sdk.AccAddress, err error) {
	if len(strings.TrimSpace(address)) == 0 {
		return sdk.AccAddress{}, fmt.Errorf("empty address string is not allowed")
	}

	bz, err := sdk.GetFromBech32(address, bech32prefix)
	if err != nil {
		return nil, err
	}

	err = sdk.VerifyAddressFormat(bz)
	if err != nil {
		return nil, err
	}

	return sdk.AccAddress(bz), nil
}

// GetIBCDenomAddress returns the address from the hash of the ICS20's Denom Path.
func GetIBCDenomAddress(denom string) (common.Address, error) {
	if !strings.HasPrefix(denom, "ibc/") {
		return common.Address{}, ibctransfertypes.ErrInvalidDenomForTransfer.Wrapf("coin %s does not have 'ibc/' prefix", denom)
	}

	if len(denom) < 5 || strings.TrimSpace(denom[4:]) == "" {
		return common.Address{}, ibctransfertypes.ErrInvalidDenomForTransfer.Wrapf("coin %s is not a valid IBC voucher hash", denom)
	}

	// Get the address from the hash of the ICS20's Denom Path
	bz, err := ibctransfertypes.ParseHexHash(denom[4:])
	if err != nil {
		return common.Address{}, ibctransfertypes.ErrInvalidDenomForTransfer.Wrap(err.Error())
	}

	return common.BytesToAddress(bz), nil
}

// SortSlice sorts a slice of any ordered type.
func SortSlice[T cmp.Ordered](slice []T) {
	sort.Slice(slice, func(i, j int) bool {
		return slice[i] < slice[j]
	})
}

func Uint256FromBigInt(i *big.Int) (*uint256.Int, error) {
	if i.Sign() < 0 {
		return nil, fmt.Errorf("trying to convert negative *big.Int (%d) to uint256.Int", i)
	}
	result, overflow := uint256.FromBig(i)
	if overflow {
		return nil, fmt.Errorf("overflow trying to convert *big.Int (%d) to uint256.Int (%s)", i, result)
	}
	return result, nil
}

// CalcBaseFee calculates the basefee of the header.
func CalcBaseFee(config *params.ChainConfig, parent *ethtypes.Header, p feemarkettypes.Params) (*big.Int, error) {
	// If the current block is the first EIP-1559 block, return the InitialBaseFee.
	if !config.IsLondon(parent.Number) {
		return new(big.Int).SetUint64(params.InitialBaseFee), nil
	}
	if p.ElasticityMultiplier == 0 {
		return nil, errors.New("ElasticityMultiplier cannot be 0 as it's checked in the params validation")
	}
	parentGasTarget := parent.GasLimit / uint64(p.ElasticityMultiplier)

	factor := evmtypes.GetEVMCoinDecimals().ConversionFactor()
	minGasPrice := p.MinGasPrice.Mul(sdkmath.LegacyNewDecFromInt(factor))
	return CalcGasBaseFee(
		parent.GasUsed, parentGasTarget, uint64(p.BaseFeeChangeDenominator),
		sdkmath.LegacyNewDecFromBigInt(parent.BaseFee), sdkmath.LegacyOneDec(), minGasPrice,
	).TruncateInt().BigInt(), nil
}

func CalcGasBaseFee(gasUsed, gasTarget, baseFeeChangeDenom uint64, baseFee, minUnitGas, minGasPrice sdkmath.LegacyDec) sdkmath.LegacyDec {
	// If the parent gasUsed is the same as the target, the baseFee remains unchanged.
	if gasUsed == gasTarget {
		return baseFee
	}

	if gasTarget == 0 {
		return sdkmath.LegacyZeroDec()
	}

	num := sdkmath.LegacyNewDecFromInt(sdkmath.NewIntFromUint64(gasUsed).Sub(sdkmath.NewIntFromUint64(gasTarget)).Abs())
	num = num.Mul(baseFee)
	num = num.QuoInt(sdkmath.NewIntFromUint64(gasTarget))
	num = num.QuoInt(sdkmath.NewIntFromUint64(baseFeeChangeDenom))

	if gasUsed > gasTarget {
		// If the parent block used more gas than its target, the baseFee should increase.
		// max(1, parentBaseFee * gasUsedDelta / parentGasTarget / baseFeeChangeDenominator)
		baseFeeDelta := sdkmath.LegacyMaxDec(num, minUnitGas)
		return baseFee.Add(baseFeeDelta)
	}

	// Otherwise if the parent block used less gas than its target, the baseFee should decrease.
	// max(minGasPrice, parentBaseFee * gasUsedDelta / parentGasTarget / baseFeeChangeDenominator)
	return sdkmath.LegacyMaxDec(baseFee.Sub(num), minGasPrice)
}

// Bytes32ToString converts a bytes32 value to string by trimming null bytes
func Bytes32ToString(data [32]byte) string {
	// Find the first null byte
	var i int
	for i = 0; i < len(data); i++ {
		if data[i] == 0 {
			break
		}
	}
	return string(data[:i])
}
