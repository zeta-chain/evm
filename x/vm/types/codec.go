package types

import (
	legacyevm "github.com/cosmos/evm/legacy/evm"
	proto "github.com/cosmos/gogoproto/proto"

	errorsmod "cosmossdk.io/errors"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
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
	registry.RegisterInterface(
		"os.vm.v1.TxData",
		(*TxData)(nil),
		&DynamicFeeTx{},
		&AccessListTx{},
		&LegacyTx{},
		&legacyevm.DynamicFeeTx{},
		&legacyevm.AccessListTx{},
		&legacyevm.LegacyTx{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

// PackTxData constructs a new Any packed with the given tx data value. It returns
// an error if the client state can't be casted to a protobuf message or if the concrete
// implementation is not registered to the protobuf codec.
func PackTxData(txData TxData) (*codectypes.Any, error) {
	msg, ok := txData.(proto.Message)
	if !ok {
		return nil, errorsmod.Wrapf(errortypes.ErrPackAny, "cannot proto marshal %T", txData)
	}

	anyTxData, err := codectypes.NewAnyWithValue(msg)
	if err != nil {
		return nil, errorsmod.Wrap(errortypes.ErrPackAny, err.Error())
	}

	return anyTxData, nil
}

// UnpackTxData unpacks an Any into a TxData. It returns an error if the
// client state can't be unpacked into a TxData.
func UnpackTxData(anyTxData *codectypes.Any) (TxData, error) {
	if anyTxData == nil {
		return nil, errorsmod.Wrap(errortypes.ErrUnpackAny, "protobuf Any message cannot be nil")
	}

	txData, ok := anyTxData.GetCachedValue().(TxData)
	if !ok {
		return nil, errorsmod.Wrapf(errortypes.ErrUnpackAny, "cannot unpack Any into TxData %T", anyTxData)
	}

	return txData, nil
}

// RegisterLegacyAminoCodec required for EIP-712
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgUpdateParams{}, updateParamsName, nil)
	cdc.RegisterConcrete(&legacyevm.MsgUpdateParams{}, legacyUpdateParamsName, nil)
}
