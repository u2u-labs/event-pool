// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package sessionreceipt

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// LibSessionReceiptSessionReceipt is an auto generated low-level Go binding around an user-defined struct.
type LibSessionReceiptSessionReceipt struct {
	Client             common.Address
	Node               common.Address
	TotalSecondsServed *big.Int
	TokenType          uint8
	TokenAddress       common.Address
	Status             uint8
	Nonce              *big.Int
}

// SessionReceiptMetaData contains all meta data concerning the SessionReceipt contract.
var SessionReceiptMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address\",\"name\":\"_nodesStorage\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"_usageDepositor\",\"type\":\"address\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"client\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"node\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"nonce\",\"type\":\"uint256\"}],\"name\":\"SessionReceiptConfirmed\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":false,\"internalType\":\"address\",\"name\":\"client\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"node\",\"type\":\"address\"},{\"indexed\":false,\"internalType\":\"uint256\",\"name\":\"totalSecondsServed\",\"type\":\"uint256\"},{\"indexed\":false,\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"}],\"name\":\"SessionReceiptCreated\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"uint256\",\"name\":\"nonce\",\"type\":\"uint256\"}],\"name\":\"confirmSessionReceipt\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"client\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"totalSecondsServed\",\"type\":\"uint256\"},{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"},{\"internalType\":\"enumTokenType\",\"name\":\"tokenType\",\"type\":\"uint8\"},{\"internalType\":\"uint256\",\"name\":\"nonce\",\"type\":\"uint256\"}],\"name\":\"createSessionReceipt\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"client\",\"type\":\"address\"}],\"name\":\"getLatestReceipt\",\"outputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"client\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"node\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"totalSecondsServed\",\"type\":\"uint256\"},{\"internalType\":\"enumTokenType\",\"name\":\"tokenType\",\"type\":\"uint8\"},{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"},{\"internalType\":\"enumLibSessionReceipt.SessionReceiptStatus\",\"name\":\"status\",\"type\":\"uint8\"},{\"internalType\":\"uint256\",\"name\":\"nonce\",\"type\":\"uint256\"}],\"internalType\":\"structLibSessionReceipt.SessionReceipt\",\"name\":\"receipt\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"client\",\"type\":\"address\"}],\"name\":\"getNonce\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"client\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"nonce\",\"type\":\"uint256\"}],\"name\":\"getSessionReceipt\",\"outputs\":[{\"components\":[{\"internalType\":\"address\",\"name\":\"client\",\"type\":\"address\"},{\"internalType\":\"address\",\"name\":\"node\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"totalSecondsServed\",\"type\":\"uint256\"},{\"internalType\":\"enumTokenType\",\"name\":\"tokenType\",\"type\":\"uint8\"},{\"internalType\":\"address\",\"name\":\"tokenAddress\",\"type\":\"address\"},{\"internalType\":\"enumLibSessionReceipt.SessionReceiptStatus\",\"name\":\"status\",\"type\":\"uint8\"},{\"internalType\":\"uint256\",\"name\":\"nonce\",\"type\":\"uint256\"}],\"internalType\":\"structLibSessionReceipt.SessionReceipt\",\"name\":\"\",\"type\":\"tuple\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"nodesStorage\",\"outputs\":[{\"internalType\":\"contractINodesStorage\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"client\",\"type\":\"address\"},{\"internalType\":\"uint256\",\"name\":\"nonce\",\"type\":\"uint256\"}],\"name\":\"redeemReceipt\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"usageDepositor\",\"outputs\":[{\"internalType\":\"contractIUsageDepositor\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"}]",
}

// SessionReceiptABI is the input ABI used to generate the binding from.
// Deprecated: Use SessionReceiptMetaData.ABI instead.
var SessionReceiptABI = SessionReceiptMetaData.ABI

// SessionReceipt is an auto generated Go binding around an Ethereum contract.
type SessionReceipt struct {
	SessionReceiptCaller     // Read-only binding to the contract
	SessionReceiptTransactor // Write-only binding to the contract
	SessionReceiptFilterer   // Log filterer for contract events
}

// SessionReceiptCaller is an auto generated read-only Go binding around an Ethereum contract.
type SessionReceiptCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SessionReceiptTransactor is an auto generated write-only Go binding around an Ethereum contract.
type SessionReceiptTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SessionReceiptFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type SessionReceiptFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// SessionReceiptSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type SessionReceiptSession struct {
	Contract     *SessionReceipt   // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// SessionReceiptCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type SessionReceiptCallerSession struct {
	Contract *SessionReceiptCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts         // Call options to use throughout this session
}

// SessionReceiptTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type SessionReceiptTransactorSession struct {
	Contract     *SessionReceiptTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts         // Transaction auth options to use throughout this session
}

// SessionReceiptRaw is an auto generated low-level Go binding around an Ethereum contract.
type SessionReceiptRaw struct {
	Contract *SessionReceipt // Generic contract binding to access the raw methods on
}

// SessionReceiptCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type SessionReceiptCallerRaw struct {
	Contract *SessionReceiptCaller // Generic read-only contract binding to access the raw methods on
}

// SessionReceiptTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type SessionReceiptTransactorRaw struct {
	Contract *SessionReceiptTransactor // Generic write-only contract binding to access the raw methods on
}

// NewSessionReceipt creates a new instance of SessionReceipt, bound to a specific deployed contract.
func NewSessionReceipt(address common.Address, backend bind.ContractBackend) (*SessionReceipt, error) {
	contract, err := bindSessionReceipt(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &SessionReceipt{SessionReceiptCaller: SessionReceiptCaller{contract: contract}, SessionReceiptTransactor: SessionReceiptTransactor{contract: contract}, SessionReceiptFilterer: SessionReceiptFilterer{contract: contract}}, nil
}

// NewSessionReceiptCaller creates a new read-only instance of SessionReceipt, bound to a specific deployed contract.
func NewSessionReceiptCaller(address common.Address, caller bind.ContractCaller) (*SessionReceiptCaller, error) {
	contract, err := bindSessionReceipt(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &SessionReceiptCaller{contract: contract}, nil
}

// NewSessionReceiptTransactor creates a new write-only instance of SessionReceipt, bound to a specific deployed contract.
func NewSessionReceiptTransactor(address common.Address, transactor bind.ContractTransactor) (*SessionReceiptTransactor, error) {
	contract, err := bindSessionReceipt(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &SessionReceiptTransactor{contract: contract}, nil
}

// NewSessionReceiptFilterer creates a new log filterer instance of SessionReceipt, bound to a specific deployed contract.
func NewSessionReceiptFilterer(address common.Address, filterer bind.ContractFilterer) (*SessionReceiptFilterer, error) {
	contract, err := bindSessionReceipt(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &SessionReceiptFilterer{contract: contract}, nil
}

// bindSessionReceipt binds a generic wrapper to an already deployed contract.
func bindSessionReceipt(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := SessionReceiptMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_SessionReceipt *SessionReceiptRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _SessionReceipt.Contract.SessionReceiptCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_SessionReceipt *SessionReceiptRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SessionReceipt.Contract.SessionReceiptTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_SessionReceipt *SessionReceiptRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _SessionReceipt.Contract.SessionReceiptTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_SessionReceipt *SessionReceiptCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _SessionReceipt.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_SessionReceipt *SessionReceiptTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SessionReceipt.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_SessionReceipt *SessionReceiptTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _SessionReceipt.Contract.contract.Transact(opts, method, params...)
}

// GetLatestReceipt is a free data retrieval call binding the contract method 0xf2f8f940.
//
// Solidity: function getLatestReceipt(address client) view returns((address,address,uint256,uint8,address,uint8,uint256) receipt)
func (_SessionReceipt *SessionReceiptCaller) GetLatestReceipt(opts *bind.CallOpts, client common.Address) (LibSessionReceiptSessionReceipt, error) {
	var out []interface{}
	err := _SessionReceipt.contract.Call(opts, &out, "getLatestReceipt", client)

	if err != nil {
		return *new(LibSessionReceiptSessionReceipt), err
	}

	out0 := *abi.ConvertType(out[0], new(LibSessionReceiptSessionReceipt)).(*LibSessionReceiptSessionReceipt)

	return out0, err

}

// GetLatestReceipt is a free data retrieval call binding the contract method 0xf2f8f940.
//
// Solidity: function getLatestReceipt(address client) view returns((address,address,uint256,uint8,address,uint8,uint256) receipt)
func (_SessionReceipt *SessionReceiptSession) GetLatestReceipt(client common.Address) (LibSessionReceiptSessionReceipt, error) {
	return _SessionReceipt.Contract.GetLatestReceipt(&_SessionReceipt.CallOpts, client)
}

// GetLatestReceipt is a free data retrieval call binding the contract method 0xf2f8f940.
//
// Solidity: function getLatestReceipt(address client) view returns((address,address,uint256,uint8,address,uint8,uint256) receipt)
func (_SessionReceipt *SessionReceiptCallerSession) GetLatestReceipt(client common.Address) (LibSessionReceiptSessionReceipt, error) {
	return _SessionReceipt.Contract.GetLatestReceipt(&_SessionReceipt.CallOpts, client)
}

// GetNonce is a free data retrieval call binding the contract method 0x2d0335ab.
//
// Solidity: function getNonce(address client) view returns(uint256)
func (_SessionReceipt *SessionReceiptCaller) GetNonce(opts *bind.CallOpts, client common.Address) (*big.Int, error) {
	var out []interface{}
	err := _SessionReceipt.contract.Call(opts, &out, "getNonce", client)

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetNonce is a free data retrieval call binding the contract method 0x2d0335ab.
//
// Solidity: function getNonce(address client) view returns(uint256)
func (_SessionReceipt *SessionReceiptSession) GetNonce(client common.Address) (*big.Int, error) {
	return _SessionReceipt.Contract.GetNonce(&_SessionReceipt.CallOpts, client)
}

// GetNonce is a free data retrieval call binding the contract method 0x2d0335ab.
//
// Solidity: function getNonce(address client) view returns(uint256)
func (_SessionReceipt *SessionReceiptCallerSession) GetNonce(client common.Address) (*big.Int, error) {
	return _SessionReceipt.Contract.GetNonce(&_SessionReceipt.CallOpts, client)
}

// GetSessionReceipt is a free data retrieval call binding the contract method 0x8eb59f8b.
//
// Solidity: function getSessionReceipt(address client, uint256 nonce) view returns((address,address,uint256,uint8,address,uint8,uint256))
func (_SessionReceipt *SessionReceiptCaller) GetSessionReceipt(opts *bind.CallOpts, client common.Address, nonce *big.Int) (LibSessionReceiptSessionReceipt, error) {
	var out []interface{}
	err := _SessionReceipt.contract.Call(opts, &out, "getSessionReceipt", client, nonce)

	if err != nil {
		return *new(LibSessionReceiptSessionReceipt), err
	}

	out0 := *abi.ConvertType(out[0], new(LibSessionReceiptSessionReceipt)).(*LibSessionReceiptSessionReceipt)

	return out0, err

}

// GetSessionReceipt is a free data retrieval call binding the contract method 0x8eb59f8b.
//
// Solidity: function getSessionReceipt(address client, uint256 nonce) view returns((address,address,uint256,uint8,address,uint8,uint256))
func (_SessionReceipt *SessionReceiptSession) GetSessionReceipt(client common.Address, nonce *big.Int) (LibSessionReceiptSessionReceipt, error) {
	return _SessionReceipt.Contract.GetSessionReceipt(&_SessionReceipt.CallOpts, client, nonce)
}

// GetSessionReceipt is a free data retrieval call binding the contract method 0x8eb59f8b.
//
// Solidity: function getSessionReceipt(address client, uint256 nonce) view returns((address,address,uint256,uint8,address,uint8,uint256))
func (_SessionReceipt *SessionReceiptCallerSession) GetSessionReceipt(client common.Address, nonce *big.Int) (LibSessionReceiptSessionReceipt, error) {
	return _SessionReceipt.Contract.GetSessionReceipt(&_SessionReceipt.CallOpts, client, nonce)
}

// NodesStorage is a free data retrieval call binding the contract method 0xe244c4ff.
//
// Solidity: function nodesStorage() view returns(address)
func (_SessionReceipt *SessionReceiptCaller) NodesStorage(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _SessionReceipt.contract.Call(opts, &out, "nodesStorage")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// NodesStorage is a free data retrieval call binding the contract method 0xe244c4ff.
//
// Solidity: function nodesStorage() view returns(address)
func (_SessionReceipt *SessionReceiptSession) NodesStorage() (common.Address, error) {
	return _SessionReceipt.Contract.NodesStorage(&_SessionReceipt.CallOpts)
}

// NodesStorage is a free data retrieval call binding the contract method 0xe244c4ff.
//
// Solidity: function nodesStorage() view returns(address)
func (_SessionReceipt *SessionReceiptCallerSession) NodesStorage() (common.Address, error) {
	return _SessionReceipt.Contract.NodesStorage(&_SessionReceipt.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_SessionReceipt *SessionReceiptCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _SessionReceipt.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_SessionReceipt *SessionReceiptSession) Owner() (common.Address, error) {
	return _SessionReceipt.Contract.Owner(&_SessionReceipt.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_SessionReceipt *SessionReceiptCallerSession) Owner() (common.Address, error) {
	return _SessionReceipt.Contract.Owner(&_SessionReceipt.CallOpts)
}

// UsageDepositor is a free data retrieval call binding the contract method 0x64cf88c2.
//
// Solidity: function usageDepositor() view returns(address)
func (_SessionReceipt *SessionReceiptCaller) UsageDepositor(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _SessionReceipt.contract.Call(opts, &out, "usageDepositor")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// UsageDepositor is a free data retrieval call binding the contract method 0x64cf88c2.
//
// Solidity: function usageDepositor() view returns(address)
func (_SessionReceipt *SessionReceiptSession) UsageDepositor() (common.Address, error) {
	return _SessionReceipt.Contract.UsageDepositor(&_SessionReceipt.CallOpts)
}

// UsageDepositor is a free data retrieval call binding the contract method 0x64cf88c2.
//
// Solidity: function usageDepositor() view returns(address)
func (_SessionReceipt *SessionReceiptCallerSession) UsageDepositor() (common.Address, error) {
	return _SessionReceipt.Contract.UsageDepositor(&_SessionReceipt.CallOpts)
}

// ConfirmSessionReceipt is a paid mutator transaction binding the contract method 0xe5483f5c.
//
// Solidity: function confirmSessionReceipt(uint256 nonce) returns()
func (_SessionReceipt *SessionReceiptTransactor) ConfirmSessionReceipt(opts *bind.TransactOpts, nonce *big.Int) (*types.Transaction, error) {
	return _SessionReceipt.contract.Transact(opts, "confirmSessionReceipt", nonce)
}

// ConfirmSessionReceipt is a paid mutator transaction binding the contract method 0xe5483f5c.
//
// Solidity: function confirmSessionReceipt(uint256 nonce) returns()
func (_SessionReceipt *SessionReceiptSession) ConfirmSessionReceipt(nonce *big.Int) (*types.Transaction, error) {
	return _SessionReceipt.Contract.ConfirmSessionReceipt(&_SessionReceipt.TransactOpts, nonce)
}

// ConfirmSessionReceipt is a paid mutator transaction binding the contract method 0xe5483f5c.
//
// Solidity: function confirmSessionReceipt(uint256 nonce) returns()
func (_SessionReceipt *SessionReceiptTransactorSession) ConfirmSessionReceipt(nonce *big.Int) (*types.Transaction, error) {
	return _SessionReceipt.Contract.ConfirmSessionReceipt(&_SessionReceipt.TransactOpts, nonce)
}

// CreateSessionReceipt is a paid mutator transaction binding the contract method 0x760643b2.
//
// Solidity: function createSessionReceipt(address client, uint256 totalSecondsServed, address tokenAddress, uint8 tokenType, uint256 nonce) returns()
func (_SessionReceipt *SessionReceiptTransactor) CreateSessionReceipt(opts *bind.TransactOpts, client common.Address, totalSecondsServed *big.Int, tokenAddress common.Address, tokenType uint8, nonce *big.Int) (*types.Transaction, error) {
	return _SessionReceipt.contract.Transact(opts, "createSessionReceipt", client, totalSecondsServed, tokenAddress, tokenType, nonce)
}

// CreateSessionReceipt is a paid mutator transaction binding the contract method 0x760643b2.
//
// Solidity: function createSessionReceipt(address client, uint256 totalSecondsServed, address tokenAddress, uint8 tokenType, uint256 nonce) returns()
func (_SessionReceipt *SessionReceiptSession) CreateSessionReceipt(client common.Address, totalSecondsServed *big.Int, tokenAddress common.Address, tokenType uint8, nonce *big.Int) (*types.Transaction, error) {
	return _SessionReceipt.Contract.CreateSessionReceipt(&_SessionReceipt.TransactOpts, client, totalSecondsServed, tokenAddress, tokenType, nonce)
}

// CreateSessionReceipt is a paid mutator transaction binding the contract method 0x760643b2.
//
// Solidity: function createSessionReceipt(address client, uint256 totalSecondsServed, address tokenAddress, uint8 tokenType, uint256 nonce) returns()
func (_SessionReceipt *SessionReceiptTransactorSession) CreateSessionReceipt(client common.Address, totalSecondsServed *big.Int, tokenAddress common.Address, tokenType uint8, nonce *big.Int) (*types.Transaction, error) {
	return _SessionReceipt.Contract.CreateSessionReceipt(&_SessionReceipt.TransactOpts, client, totalSecondsServed, tokenAddress, tokenType, nonce)
}

// RedeemReceipt is a paid mutator transaction binding the contract method 0x55d3c58a.
//
// Solidity: function redeemReceipt(address client, uint256 nonce) returns()
func (_SessionReceipt *SessionReceiptTransactor) RedeemReceipt(opts *bind.TransactOpts, client common.Address, nonce *big.Int) (*types.Transaction, error) {
	return _SessionReceipt.contract.Transact(opts, "redeemReceipt", client, nonce)
}

// RedeemReceipt is a paid mutator transaction binding the contract method 0x55d3c58a.
//
// Solidity: function redeemReceipt(address client, uint256 nonce) returns()
func (_SessionReceipt *SessionReceiptSession) RedeemReceipt(client common.Address, nonce *big.Int) (*types.Transaction, error) {
	return _SessionReceipt.Contract.RedeemReceipt(&_SessionReceipt.TransactOpts, client, nonce)
}

// RedeemReceipt is a paid mutator transaction binding the contract method 0x55d3c58a.
//
// Solidity: function redeemReceipt(address client, uint256 nonce) returns()
func (_SessionReceipt *SessionReceiptTransactorSession) RedeemReceipt(client common.Address, nonce *big.Int) (*types.Transaction, error) {
	return _SessionReceipt.Contract.RedeemReceipt(&_SessionReceipt.TransactOpts, client, nonce)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_SessionReceipt *SessionReceiptTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _SessionReceipt.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_SessionReceipt *SessionReceiptSession) RenounceOwnership() (*types.Transaction, error) {
	return _SessionReceipt.Contract.RenounceOwnership(&_SessionReceipt.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_SessionReceipt *SessionReceiptTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _SessionReceipt.Contract.RenounceOwnership(&_SessionReceipt.TransactOpts)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_SessionReceipt *SessionReceiptTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _SessionReceipt.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_SessionReceipt *SessionReceiptSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _SessionReceipt.Contract.TransferOwnership(&_SessionReceipt.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_SessionReceipt *SessionReceiptTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _SessionReceipt.Contract.TransferOwnership(&_SessionReceipt.TransactOpts, newOwner)
}

// SessionReceiptOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the SessionReceipt contract.
type SessionReceiptOwnershipTransferredIterator struct {
	Event *SessionReceiptOwnershipTransferred // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SessionReceiptOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SessionReceiptOwnershipTransferred)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SessionReceiptOwnershipTransferred)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SessionReceiptOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SessionReceiptOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SessionReceiptOwnershipTransferred represents a OwnershipTransferred event raised by the SessionReceipt contract.
type SessionReceiptOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_SessionReceipt *SessionReceiptFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*SessionReceiptOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _SessionReceipt.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &SessionReceiptOwnershipTransferredIterator{contract: _SessionReceipt.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_SessionReceipt *SessionReceiptFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *SessionReceiptOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _SessionReceipt.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SessionReceiptOwnershipTransferred)
				if err := _SessionReceipt.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseOwnershipTransferred is a log parse operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_SessionReceipt *SessionReceiptFilterer) ParseOwnershipTransferred(log types.Log) (*SessionReceiptOwnershipTransferred, error) {
	event := new(SessionReceiptOwnershipTransferred)
	if err := _SessionReceipt.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SessionReceiptSessionReceiptConfirmedIterator is returned from FilterSessionReceiptConfirmed and is used to iterate over the raw logs and unpacked data for SessionReceiptConfirmed events raised by the SessionReceipt contract.
type SessionReceiptSessionReceiptConfirmedIterator struct {
	Event *SessionReceiptSessionReceiptConfirmed // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SessionReceiptSessionReceiptConfirmedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SessionReceiptSessionReceiptConfirmed)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SessionReceiptSessionReceiptConfirmed)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SessionReceiptSessionReceiptConfirmedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SessionReceiptSessionReceiptConfirmedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SessionReceiptSessionReceiptConfirmed represents a SessionReceiptConfirmed event raised by the SessionReceipt contract.
type SessionReceiptSessionReceiptConfirmed struct {
	Client common.Address
	Node   common.Address
	Nonce  *big.Int
	Raw    types.Log // Blockchain specific contextual infos
}

// FilterSessionReceiptConfirmed is a free log retrieval operation binding the contract event 0xd56648885e56047e04b1f631de6afaa1f1b81eeeb672ba27ebaafe44db00c1e6.
//
// Solidity: event SessionReceiptConfirmed(address client, address node, uint256 nonce)
func (_SessionReceipt *SessionReceiptFilterer) FilterSessionReceiptConfirmed(opts *bind.FilterOpts) (*SessionReceiptSessionReceiptConfirmedIterator, error) {

	logs, sub, err := _SessionReceipt.contract.FilterLogs(opts, "SessionReceiptConfirmed")
	if err != nil {
		return nil, err
	}
	return &SessionReceiptSessionReceiptConfirmedIterator{contract: _SessionReceipt.contract, event: "SessionReceiptConfirmed", logs: logs, sub: sub}, nil
}

// WatchSessionReceiptConfirmed is a free log subscription operation binding the contract event 0xd56648885e56047e04b1f631de6afaa1f1b81eeeb672ba27ebaafe44db00c1e6.
//
// Solidity: event SessionReceiptConfirmed(address client, address node, uint256 nonce)
func (_SessionReceipt *SessionReceiptFilterer) WatchSessionReceiptConfirmed(opts *bind.WatchOpts, sink chan<- *SessionReceiptSessionReceiptConfirmed) (event.Subscription, error) {

	logs, sub, err := _SessionReceipt.contract.WatchLogs(opts, "SessionReceiptConfirmed")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SessionReceiptSessionReceiptConfirmed)
				if err := _SessionReceipt.contract.UnpackLog(event, "SessionReceiptConfirmed", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseSessionReceiptConfirmed is a log parse operation binding the contract event 0xd56648885e56047e04b1f631de6afaa1f1b81eeeb672ba27ebaafe44db00c1e6.
//
// Solidity: event SessionReceiptConfirmed(address client, address node, uint256 nonce)
func (_SessionReceipt *SessionReceiptFilterer) ParseSessionReceiptConfirmed(log types.Log) (*SessionReceiptSessionReceiptConfirmed, error) {
	event := new(SessionReceiptSessionReceiptConfirmed)
	if err := _SessionReceipt.contract.UnpackLog(event, "SessionReceiptConfirmed", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// SessionReceiptSessionReceiptCreatedIterator is returned from FilterSessionReceiptCreated and is used to iterate over the raw logs and unpacked data for SessionReceiptCreated events raised by the SessionReceipt contract.
type SessionReceiptSessionReceiptCreatedIterator struct {
	Event *SessionReceiptSessionReceiptCreated // Event containing the contract specifics and raw log

	contract *bind.BoundContract // Generic contract to use for unpacking event data
	event    string              // Event name to use for unpacking event data

	logs chan types.Log        // Log channel receiving the found contract events
	sub  ethereum.Subscription // Subscription for errors, completion and termination
	done bool                  // Whether the subscription completed delivering logs
	fail error                 // Occurred error to stop iteration
}

// Next advances the iterator to the subsequent event, returning whether there
// are any more events found. In case of a retrieval or parsing error, false is
// returned and Error() can be queried for the exact failure.
func (it *SessionReceiptSessionReceiptCreatedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(SessionReceiptSessionReceiptCreated)
			if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
				it.fail = err
				return false
			}
			it.Event.Raw = log
			return true

		default:
			return false
		}
	}
	// Iterator still in progress, wait for either a data or an error event
	select {
	case log := <-it.logs:
		it.Event = new(SessionReceiptSessionReceiptCreated)
		if err := it.contract.UnpackLog(it.Event, it.event, log); err != nil {
			it.fail = err
			return false
		}
		it.Event.Raw = log
		return true

	case err := <-it.sub.Err():
		it.done = true
		it.fail = err
		return it.Next()
	}
}

// Error returns any retrieval or parsing error occurred during filtering.
func (it *SessionReceiptSessionReceiptCreatedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *SessionReceiptSessionReceiptCreatedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// SessionReceiptSessionReceiptCreated represents a SessionReceiptCreated event raised by the SessionReceipt contract.
type SessionReceiptSessionReceiptCreated struct {
	Client             common.Address
	Node               common.Address
	TotalSecondsServed *big.Int
	TokenAddress       common.Address
	Raw                types.Log // Blockchain specific contextual infos
}

// FilterSessionReceiptCreated is a free log retrieval operation binding the contract event 0x5b41fa5a246197b0d07600ec73bc1049ffe75eb19383fdd8664b0f37e057c117.
//
// Solidity: event SessionReceiptCreated(address client, address node, uint256 totalSecondsServed, address tokenAddress)
func (_SessionReceipt *SessionReceiptFilterer) FilterSessionReceiptCreated(opts *bind.FilterOpts) (*SessionReceiptSessionReceiptCreatedIterator, error) {

	logs, sub, err := _SessionReceipt.contract.FilterLogs(opts, "SessionReceiptCreated")
	if err != nil {
		return nil, err
	}
	return &SessionReceiptSessionReceiptCreatedIterator{contract: _SessionReceipt.contract, event: "SessionReceiptCreated", logs: logs, sub: sub}, nil
}

// WatchSessionReceiptCreated is a free log subscription operation binding the contract event 0x5b41fa5a246197b0d07600ec73bc1049ffe75eb19383fdd8664b0f37e057c117.
//
// Solidity: event SessionReceiptCreated(address client, address node, uint256 totalSecondsServed, address tokenAddress)
func (_SessionReceipt *SessionReceiptFilterer) WatchSessionReceiptCreated(opts *bind.WatchOpts, sink chan<- *SessionReceiptSessionReceiptCreated) (event.Subscription, error) {

	logs, sub, err := _SessionReceipt.contract.WatchLogs(opts, "SessionReceiptCreated")
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(SessionReceiptSessionReceiptCreated)
				if err := _SessionReceipt.contract.UnpackLog(event, "SessionReceiptCreated", log); err != nil {
					return err
				}
				event.Raw = log

				select {
				case sink <- event:
				case err := <-sub.Err():
					return err
				case <-quit:
					return nil
				}
			case err := <-sub.Err():
				return err
			case <-quit:
				return nil
			}
		}
	}), nil
}

// ParseSessionReceiptCreated is a log parse operation binding the contract event 0x5b41fa5a246197b0d07600ec73bc1049ffe75eb19383fdd8664b0f37e057c117.
//
// Solidity: event SessionReceiptCreated(address client, address node, uint256 totalSecondsServed, address tokenAddress)
func (_SessionReceipt *SessionReceiptFilterer) ParseSessionReceiptCreated(log types.Log) (*SessionReceiptSessionReceiptCreated, error) {
	event := new(SessionReceiptSessionReceiptCreated)
	if err := _SessionReceipt.contract.UnpackLog(event, "SessionReceiptCreated", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
