package evm

import (
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"math/big"
)

var _ TxData = &AccessListTx{}

func (tx *AccessListTx) TxType() byte {
	return 0x00
}

func (tx *AccessListTx) Copy() TxData {
	return nil
}

func (tx *AccessListTx) GetChainID() *big.Int {
	return nil
}

func (tx *AccessListTx) GetAccessList() ethtypes.AccessList {
	return nil // Legacy transactions don't have access lists
}

func (tx *AccessListTx) GetData() []byte {
	return []byte{}
}

func (tx *AccessListTx) GetNonce() uint64 {
	return 0
}

func (tx *AccessListTx) GetGas() uint64 {
	return 0
}

func (tx *AccessListTx) GetGasPrice() *big.Int {
	return nil
}

func (tx *AccessListTx) GetGasTipCap() *big.Int {
	return nil
}

func (tx *AccessListTx) GetGasFeeCap() *big.Int {
	return nil
}

func (tx *AccessListTx) GetValue() *big.Int {
	return nil
}

func (tx *AccessListTx) GetTo() *common.Address {
	return nil
}

func (tx *AccessListTx) GetRawSignatureValues() (v, r, s *big.Int) {
	return nil, nil, nil
}

func (tx *AccessListTx) SetSignatureValues(_, _, _, _ *big.Int) {}

func (tx *AccessListTx) AsEthereumData() ethtypes.TxData {
	return nil
}

func (tx *AccessListTx) Validate() error {
	return nil
}

func (tx *AccessListTx) Fee() *big.Int {
	return nil
}

func (tx *AccessListTx) Cost() *big.Int {
	return nil
}

func (tx *AccessListTx) EffectiveGasPrice(_ *big.Int) *big.Int {
	return nil
}

func (tx *AccessListTx) EffectiveFee(_ *big.Int) *big.Int {
	return nil
}

func (tx *AccessListTx) EffectiveCost(_ *big.Int) *big.Int {
	return nil
}
