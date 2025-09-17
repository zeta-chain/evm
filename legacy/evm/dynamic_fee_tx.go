package evm

import (
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"math/big"
)

func (tx *DynamicFeeTx) TxType() byte {
	return 0x00
}

func (tx *DynamicFeeTx) Copy() TxData {
	return nil
}

func (tx *DynamicFeeTx) GetChainID() *big.Int {
	return nil
}

func (tx *DynamicFeeTx) GetAccessList() ethtypes.AccessList {
	return nil
}

func (tx *DynamicFeeTx) GetData() []byte {
	return []byte{}
}

func (tx *DynamicFeeTx) GetNonce() uint64 {
	return 0
}

func (tx *DynamicFeeTx) GetGas() uint64 {
	return 0
}

func (tx *DynamicFeeTx) GetGasPrice() *big.Int {
	return nil
}

func (tx *DynamicFeeTx) GetGasTipCap() *big.Int {
	return nil
}

func (tx *DynamicFeeTx) GetGasFeeCap() *big.Int {
	return nil
}

func (tx *DynamicFeeTx) GetValue() *big.Int {
	return nil
}

func (tx *DynamicFeeTx) GetTo() *common.Address {
	return nil
}

func (tx *DynamicFeeTx) GetRawSignatureValues() (v, r, s *big.Int) {
	return nil, nil, nil
}

func (tx *DynamicFeeTx) SetSignatureValues(_, _, _, _ *big.Int) {}

func (tx *DynamicFeeTx) AsEthereumData() ethtypes.TxData {
	return nil
}

func (tx *DynamicFeeTx) Validate() error {
	return nil
}

func (tx *DynamicFeeTx) Fee() *big.Int {
	return nil
}

func (tx *DynamicFeeTx) Cost() *big.Int {
	return nil
}

func (tx *DynamicFeeTx) EffectiveGasPrice(_ *big.Int) *big.Int {
	return nil
}

func (tx *DynamicFeeTx) EffectiveFee(_ *big.Int) *big.Int {
	return nil
}

func (tx *DynamicFeeTx) EffectiveCost(_ *big.Int) *big.Int {
	return nil
}
