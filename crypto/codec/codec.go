package codec

import (
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	legacytypes "github.com/cosmos/evm/legacy/types"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

// RegisterInterfaces register the Cosmos EVM key concrete types.
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations((*cryptotypes.PubKey)(nil), &ethsecp256k1.PubKey{})
	registry.RegisterImplementations((*cryptotypes.PrivKey)(nil), &ethsecp256k1.PrivKey{})

	registry.RegisterImplementations((*cryptotypes.PubKey)(nil), &legacytypes.PubKey{})
	registry.RegisterImplementations((*cryptotypes.PrivKey)(nil), &legacytypes.PrivKey{})

}
