package types

import (
	"fmt"
	"maps"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/types"
)

// Parameter store key
var (
	ParamStoreKeyEnableErc20                = []byte("EnableErc20") // figure out where this is initialized
	ParamStoreKeyPermissionlessRegistration = []byte("PermissionlessRegistration")
)

var (
	CtxKeyDynamicPrecompiles = "DynamicPrecompiles"
	CtxKeyNativePrecompiles  = "NativePrecompiles"
)

var (
	// NOTE: We strongly recommend to use the canonical address for the ERC-20 representation
	// of the chain's native denomination as defined by
	// [ERC-7528](https://eips.ethereum.org/EIPS/eip-7528).
	//
	// 0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE
	DefaultNativePrecompiles  = make(map[string]bool)
	DefaultDynamicPrecompiles = make(map[string]bool)
)

// NewParams creates a new Params object
func NewParams(
	enableErc20 bool,
	nativePrecompiles map[string]bool,
	dynamicPrecompiles map[string]bool,
	permissionlessRegistration bool,
) Params {
	return Params{
		EnableErc20:                enableErc20,
		NativePrecompiles:          nativePrecompiles,
		DynamicPrecompiles:         dynamicPrecompiles,
		PermissionlessRegistration: permissionlessRegistration,
	}
}

func DefaultParams() Params {
	return Params{
		EnableErc20:                true,
		NativePrecompiles:          DefaultNativePrecompiles,
		DynamicPrecompiles:         DefaultDynamicPrecompiles,
		PermissionlessRegistration: true,
	}
}

func (p Params) Validate() error {
	if err := ValidatePrecompiles(p.NativePrecompiles); err != nil {
		return err
	}
	if err := ValidatePrecompiles(p.DynamicPrecompiles); err != nil {
		return err
	}

	combined := make(map[string]bool, len(p.NativePrecompiles)+len(p.DynamicPrecompiles))
	maps.Copy(p.NativePrecompiles, combined)
	maps.Copy(p.DynamicPrecompiles, combined)
	return validatePrecompilesUniqueness(combined)
}

// ValidatePrecompiles checks if the precompile addresses are valid and unique.
func ValidatePrecompiles(precompiles map[string]bool) error {
	for precompile := range precompiles {
		err := types.ValidateAddress(precompile)
		if err != nil {
			return fmt.Errorf("invalid precompile address %s", precompile)
		}
	}
	return nil
}

func validatePrecompilesUniqueness(precompiles map[string]bool) error {
	seenPrecompiles := make(map[string]struct{})
	for precompile := range precompiles {
		// use address.Hex() to make sure all addresses are using EIP-55
		eip55Addr := common.HexToAddress(precompile).Hex()
		if _, ok := seenPrecompiles[eip55Addr]; ok {
			return fmt.Errorf("duplicate precompile %s", precompile)
		}

		seenPrecompiles[eip55Addr] = struct{}{}
	}
	return nil
}

// IsNativePrecompile checks if the provided address is within the native precompiles
func (p Params) IsNativePrecompile(addr common.Address) bool {
	_, ok := p.NativePrecompiles[addr.String()]
	return ok
}

// IsDynamicPrecompile checks if the provided address is within the dynamic precompiles
func (p Params) IsDynamicPrecompile(addr common.Address) bool {
	_, ok := p.DynamicPrecompiles[addr.String()]
	return ok
}
