package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
	legacytypes "github.com/cosmos/evm/legacy/types"
)

var (
	amino = codec.NewLegacyAmino()
	// ModuleCdc references the global fee market module codec. Note, the codec should
	// ONLY be used in certain instances of tests and for JSON encoding.
	ModuleCdc = codec.NewProtoCodec(codectypes.NewInterfaceRegistry())

	// AminoCdc is an amino codec created to support amino JSON compatible msgs.
	//
	// For now, we'll leave this. PENDING to check if we can remove it.
	AminoCdc = codec.NewAminoCodec(amino) //nolint:staticcheck
)

const (
	// Amino names
	updateParamsName = "os/feemarket/MsgUpdateParams"
)

// NOTE: This is required for the GetSignBytes function
func init() {
	RegisterLegacyAminoCodec(amino)
	amino.Seal()
}

// RegisterInterfaces registers the client interfaces to protobuf Any.
func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations(
		(*sdk.Msg)(nil),
		&MsgUpdateParams{},
	)

	registry.RegisterImplementations(
		&legacytypes.MsgUpdateParams{}, // Keep interface for backwards compatibility on proposals query
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

// RegisterLegacyAminoCodec required for EIP-712
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgUpdateParams{}, updateParamsName, nil)
	cdc.RegisterConcrete(&legacytypes.MsgUpdateParams{}, "ethermint/evm.v1/MsgUpdateParams", nil)
}
