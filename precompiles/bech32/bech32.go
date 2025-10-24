package bech32

import (
	"embed"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

var _ vm.PrecompiledContract = &Precompile{}

var (
	// Embed abi json file to the executable binary. Needed when importing as dependency.
	//
	//go:embed abi.json
	f   embed.FS
	ABI abi.ABI
)

func init() {
	var err error
	ABI, err = cmn.LoadABI(f, "abi.json")
	if err != nil {
		panic(err)
	}
}

// Precompile defines the precompiled contract for Bech32 encoding.
type Precompile struct {
	abi.ABI
	baseGas uint64
}

// NewPrecompile creates a new bech32 Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(baseGas uint64) (*Precompile, error) {
	if baseGas == 0 {
		return nil, fmt.Errorf("baseGas cannot be zero")
	}

	return &Precompile{
		ABI:     ABI,
		baseGas: baseGas,
	}, nil
}

// Address defines the address of the bech32 precompiled contract.
func (Precompile) Address() common.Address {
	return common.HexToAddress(evmtypes.Bech32PrecompileAddress)
}

// RequiredGas calculates the contract gas use.
func (p Precompile) RequiredGas(_ []byte) uint64 {
	return p.baseGas
}

// Run executes the precompiled contract bech32 methods defined in the ABI.
func (p Precompile) Run(_ *vm.EVM, contract *vm.Contract, _ bool) (bz []byte, err error) {
	// NOTE: This check avoid panicking when trying to decode the method ID
	if len(contract.Input) < 4 {
		return nil, vm.ErrExecutionReverted
	}

	methodID := contract.Input[:4]
	// NOTE: this function iterates over the method map and returns
	// the method with the given ID
	method, err := p.MethodById(methodID)
	if err != nil {
		return nil, err
	}

	argsBz := contract.Input[4:]
	args, err := method.Inputs.Unpack(argsBz)
	if err != nil {
		return nil, err
	}

	switch method.Name {
	case HexToBech32Method:
		bz, err = p.HexToBech32(method, args)
	case Bech32ToHexMethod:
		bz, err = p.Bech32ToHex(method, args)
	}

	if err != nil {
		return nil, err
	}

	return bz, nil
}
