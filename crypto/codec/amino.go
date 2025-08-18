package codec

import (
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	legacysecp256k1 "github.com/cosmos/evm/legacy/ethsecp256k1"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/legacy"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
)

const (
	// PrivKeyName defines the amino encoding name for the EthSecp256k1 private key
	LegacyPrivKeyName = "ethermint/PrivKeyEthSecp256k1"
	// PubKeyName defines the amino encoding name for the EthSecp256k1 public key
	LegacyPubKeyName = "ethermint/PubKeyEthSecp256k1"
)

// RegisterCrypto registers all crypto dependency types with the provided Amino
// codec.
func RegisterCrypto(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&ethsecp256k1.PubKey{},
		ethsecp256k1.PubKeyName, nil)
	cdc.RegisterConcrete(&ethsecp256k1.PrivKey{},
		ethsecp256k1.PrivKeyName, nil)

	cdc.RegisterConcrete(&legacysecp256k1.PubKey{},
		LegacyPubKeyName, nil)
	cdc.RegisterConcrete(&legacysecp256k1.PrivKey{},
		LegacyPrivKeyName, nil)

	keyring.RegisterLegacyAminoCodec(cdc)
	cryptocodec.RegisterCrypto(cdc)

	// NOTE: update SDK's amino codec to include the ethsecp256k1 keys.
	// DO NOT REMOVE unless deprecated on the SDK.
	legacy.Cdc = cdc
}
