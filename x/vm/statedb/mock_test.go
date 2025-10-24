package statedb_test

import (
	"errors"
	"maps"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types"

	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	_          statedb.Keeper = &MockKeeper{}
	errAddress common.Address = common.BigToAddress(big.NewInt(100))
)

type MockAcount struct {
	account statedb.Account
	states  statedb.Storage
}

type MockKeeper struct {
	accounts map[common.Address]MockAcount
	codes    map[common.Hash][]byte
}

func (k MockKeeper) GetCodeHash(_ sdk.Context, addr common.Address) common.Hash {
	return common.HexToHash(addr.Hex())
}

func NewMockKeeper() *MockKeeper {
	return &MockKeeper{
		accounts: make(map[common.Address]MockAcount),
		codes:    make(map[common.Hash][]byte),
	}
}

func (k MockKeeper) GetAccount(_ sdk.Context, addr common.Address) *statedb.Account {
	acct, ok := k.accounts[addr]
	if !ok {
		return nil
	}
	return &acct.account
}

func (k MockKeeper) GetState(_ sdk.Context, addr common.Address, key common.Hash) common.Hash {
	return k.accounts[addr].states[key]
}

func (k MockKeeper) GetCode(_ sdk.Context, codeHash common.Hash) []byte {
	return k.codes[codeHash]
}

func (k MockKeeper) ForEachStorage(_ sdk.Context, addr common.Address, cb func(key, value common.Hash) bool) {
	if acct, ok := k.accounts[addr]; ok {
		for k, v := range acct.states {
			if !cb(k, v) {
				return
			}
		}
	}
}

func (k MockKeeper) SetAccount(_ sdk.Context, addr common.Address, account statedb.Account) error {
	if addr == errAddress {
		return errors.New("mock db error")
	}
	acct, exists := k.accounts[addr]
	if exists {
		// update
		acct.account = account
		k.accounts[addr] = acct
	} else {
		k.accounts[addr] = MockAcount{account: account, states: make(statedb.Storage)}
	}
	return nil
}

func (k MockKeeper) SetState(_ sdk.Context, addr common.Address, key common.Hash, value []byte) {
	if acct, ok := k.accounts[addr]; ok {
		acct.states[key] = common.BytesToHash(value)
	}
}

func (k MockKeeper) DeleteState(_ sdk.Context, addr common.Address, key common.Hash) {
	if acct, ok := k.accounts[addr]; ok {
		delete(acct.states, key)
	}
}

func (k MockKeeper) SetCode(_ sdk.Context, codeHash []byte, code []byte) {
	k.codes[common.BytesToHash(codeHash)] = code
}

func (k MockKeeper) DeleteCode(_ sdk.Context, codeHash []byte) {
	delete(k.codes, common.BytesToHash(codeHash))
}

func (k MockKeeper) DeleteAccount(_ sdk.Context, addr common.Address) error {
	if addr == errAddress {
		return errors.New("mock db error")
	}
	old := k.accounts[addr]
	delete(k.accounts, addr)
	if !types.IsEmptyCodeHash(old.account.CodeHash) {
		delete(k.codes, common.BytesToHash(old.account.CodeHash))
	}
	return nil
}

func (k MockKeeper) Clone() *MockKeeper {
	accounts := maps.Clone(k.accounts)
	codes := maps.Clone(k.codes)
	return &MockKeeper{accounts, codes}
}

func (k MockKeeper) KVStoreKeys() map[string]*storetypes.KVStoreKey {
	return make(map[string]*storetypes.KVStoreKey)
}
