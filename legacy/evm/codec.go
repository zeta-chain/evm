package evm

import (
	errorsmod "cosmossdk.io/errors"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	proto "github.com/cosmos/gogoproto/proto"
)

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
func UnpackTxData(cdcAny *codectypes.Any) (TxData, error) {
	if cdcAny == nil {
		return nil, errorsmod.Wrap(errortypes.ErrUnpackAny, "protobuf Any message cannot be nil")
	}

	txData, ok := cdcAny.GetCachedValue().(TxData)
	if !ok {
		return nil, errorsmod.Wrapf(errortypes.ErrUnpackAny, "cannot unpack Any into TxData %T", cdcAny)
	}

	return txData, nil
}
