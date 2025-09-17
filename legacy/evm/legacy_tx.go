package evm

import (
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"math/big"
)

func (tx *LegacyTx) TxType() byte {
	return 0x00
}

func (tx *LegacyTx) Copy() TxData {
	return nil
}

func (tx *LegacyTx) GetChainID() *big.Int {
	return nil
}

func (tx *LegacyTx) GetAccessList() ethtypes.AccessList {
	return nil // Legacy transactions don't have access lists
}

func (tx *LegacyTx) GetData() []byte {
	return []byte{}
}

func (tx *LegacyTx) GetNonce() uint64 {
	return 0
}

func (tx *LegacyTx) GetGas() uint64 {
	return 0
}

func (tx *LegacyTx) GetGasPrice() *big.Int {
	return nil
}

func (tx *LegacyTx) GetGasTipCap() *big.Int {
	return nil
}

func (tx *LegacyTx) GetGasFeeCap() *big.Int {
	return nil
}

func (tx *LegacyTx) GetValue() *big.Int {
	return nil
}

func (tx *LegacyTx) GetTo() *common.Address {
	return nil
}

func (tx *LegacyTx) GetRawSignatureValues() (v, r, s *big.Int) {
	return nil, nil, nil
}

func (tx *LegacyTx) SetSignatureValues(_, _, _, _ *big.Int) {}

func (tx *LegacyTx) AsEthereumData() ethtypes.TxData {
	return nil
}

func (tx *LegacyTx) Validate() error {
	return nil
}

func (tx *LegacyTx) Fee() *big.Int {
	return nil
}

func (tx *LegacyTx) Cost() *big.Int {
	return nil
}

func (tx *LegacyTx) EffectiveGasPrice(_ *big.Int) *big.Int {
	return nil
}

func (tx *LegacyTx) EffectiveFee(_ *big.Int) *big.Int {
	return nil
}

func (tx *LegacyTx) EffectiveCost(_ *big.Int) *big.Int {
	return nil
}
