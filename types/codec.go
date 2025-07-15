package types

import (
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdktypes "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	legacytypes "github.com/cosmos/evm/legacy/types"
)

// RegisterInterfaces registers the tendermint concrete client-related
// implementations and interfaces.
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations(
		(*sdktypes.AccountI)(nil),
		// TODO: uncomment after moving into migrations for EVM version
		&legacytypes.EthAccount{},
	)
	registry.RegisterImplementations(
		(*authtypes.GenesisAccount)(nil),
		// TODO: uncomment after moving into migrations for EVM version
		&legacytypes.EthAccount{},
	)
	registry.RegisterImplementations(
		(*tx.TxExtensionOptionI)(nil),
		&ExtensionOptionsWeb3Tx{},
		&ExtensionOptionDynamicFeeTx{},
	)
}
