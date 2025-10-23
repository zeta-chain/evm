package types

import (
	legacyevm "github.com/cosmos/evm/legacy/evm"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
	"github.com/cosmos/cosmos-sdk/types/tx"
)

var (
	amino = codec.NewLegacyAmino()
	// ModuleCdc references the global evm module codec. Note, the codec should
	// ONLY be used in certain instances of tests and for JSON encoding.
	ModuleCdc = codec.NewProtoCodec(codectypes.NewInterfaceRegistry())

	// AminoCdc is a amino codec created to support amino JSON compatible msgs.
	AminoCdc = codec.NewLegacyAmino()
)

const (
	// Amino names
	updateParamsName       = "os/evm/MsgUpdateParams"
	legacyUpdateParamsName = "ethermint/MsgUpdateParams"
)

// NOTE: This is required for the GetSignBytes function
func init() {
	RegisterLegacyAminoCodec(amino)
	amino.Seal()
}

// RegisterInterfaces registers the client interfaces to protobuf Any.
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations(
		(*tx.TxExtensionOptionI)(nil),
		&ExtensionOptionsEthereumTx{},
		&legacyevm.ExtensionOptionsEthereumTx{},
	)
	registry.RegisterImplementations(
		(*sdk.Msg)(nil),
		&MsgEthereumTx{},
		&MsgUpdateParams{},
		&legacyevm.MsgEthereumTx{},
		&legacyevm.MsgUpdateParams{},
	)
	// register legacy evm tx data types for backward compatibility
	registry.RegisterInterface(
		"ethermint.evm.v1.TxData",
		(*legacyevm.TxData)(nil),
		&legacyevm.DynamicFeeTx{},
		&legacyevm.AccessListTx{},
		&legacyevm.LegacyTx{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

// RegisterLegacyAminoCodec required for EIP-712
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgUpdateParams{}, updateParamsName, nil)
	cdc.RegisterConcrete(&legacyevm.MsgUpdateParams{}, legacyUpdateParamsName, nil)
}
