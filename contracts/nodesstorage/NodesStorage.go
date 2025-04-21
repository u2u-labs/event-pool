// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package nodestorage

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

// NodesStorageMetaData contains all meta data concerning the NodesStorage contract.
var NodesStorageMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"address[]\",\"name\":\"initialNodes\",\"type\":\"address[]\"}],\"stateMutability\":\"nonpayable\",\"type\":\"constructor\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"node\",\"type\":\"address\"}],\"name\":\"NodeAdded\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"node\",\"type\":\"address\"}],\"name\":\"NodeRemoved\",\"type\":\"event\"},{\"anonymous\":false,\"inputs\":[{\"indexed\":true,\"internalType\":\"address\",\"name\":\"previousOwner\",\"type\":\"address\"},{\"indexed\":true,\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"OwnershipTransferred\",\"type\":\"event\"},{\"inputs\":[{\"internalType\":\"address[]\",\"name\":\"newNodes\",\"type\":\"address[]\"}],\"name\":\"addNodes\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getTotalValidNodes\",\"outputs\":[{\"internalType\":\"uint256\",\"name\":\"total\",\"type\":\"uint256\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"getValidNodes\",\"outputs\":[{\"internalType\":\"address[]\",\"name\":\"validNodes\",\"type\":\"address[]\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"node\",\"type\":\"address\"}],\"name\":\"isValidNode\",\"outputs\":[{\"internalType\":\"bool\",\"name\":\"\",\"type\":\"bool\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"owner\",\"outputs\":[{\"internalType\":\"address\",\"name\":\"\",\"type\":\"address\"}],\"stateMutability\":\"view\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"node\",\"type\":\"address\"}],\"name\":\"removeNode\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[],\"name\":\"renounceOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"address\",\"name\":\"newOwner\",\"type\":\"address\"}],\"name\":\"transferOwnership\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// NodesStorageABI is the input ABI used to generate the binding from.
// Deprecated: Use NodesStorageMetaData.ABI instead.
var NodesStorageABI = NodesStorageMetaData.ABI

// NodesStorage is an auto generated Go binding around an Ethereum contract.
type NodesStorage struct {
	NodesStorageCaller     // Read-only binding to the contract
	NodesStorageTransactor // Write-only binding to the contract
	NodesStorageFilterer   // Log filterer for contract events
}

// NodesStorageCaller is an auto generated read-only Go binding around an Ethereum contract.
type NodesStorageCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NodesStorageTransactor is an auto generated write-only Go binding around an Ethereum contract.
type NodesStorageTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NodesStorageFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type NodesStorageFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// NodesStorageSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type NodesStorageSession struct {
	Contract     *NodesStorage     // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// NodesStorageCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type NodesStorageCallerSession struct {
	Contract *NodesStorageCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts       // Call options to use throughout this session
}

// NodesStorageTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type NodesStorageTransactorSession struct {
	Contract     *NodesStorageTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts       // Transaction auth options to use throughout this session
}

// NodesStorageRaw is an auto generated low-level Go binding around an Ethereum contract.
type NodesStorageRaw struct {
	Contract *NodesStorage // Generic contract binding to access the raw methods on
}

// NodesStorageCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type NodesStorageCallerRaw struct {
	Contract *NodesStorageCaller // Generic read-only contract binding to access the raw methods on
}

// NodesStorageTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type NodesStorageTransactorRaw struct {
	Contract *NodesStorageTransactor // Generic write-only contract binding to access the raw methods on
}

// NewNodesStorage creates a new instance of NodesStorage, bound to a specific deployed contract.
func NewNodesStorage(address common.Address, backend bind.ContractBackend) (*NodesStorage, error) {
	contract, err := bindNodesStorage(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &NodesStorage{NodesStorageCaller: NodesStorageCaller{contract: contract}, NodesStorageTransactor: NodesStorageTransactor{contract: contract}, NodesStorageFilterer: NodesStorageFilterer{contract: contract}}, nil
}

// NewNodesStorageCaller creates a new read-only instance of NodesStorage, bound to a specific deployed contract.
func NewNodesStorageCaller(address common.Address, caller bind.ContractCaller) (*NodesStorageCaller, error) {
	contract, err := bindNodesStorage(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &NodesStorageCaller{contract: contract}, nil
}

// NewNodesStorageTransactor creates a new write-only instance of NodesStorage, bound to a specific deployed contract.
func NewNodesStorageTransactor(address common.Address, transactor bind.ContractTransactor) (*NodesStorageTransactor, error) {
	contract, err := bindNodesStorage(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &NodesStorageTransactor{contract: contract}, nil
}

// NewNodesStorageFilterer creates a new log filterer instance of NodesStorage, bound to a specific deployed contract.
func NewNodesStorageFilterer(address common.Address, filterer bind.ContractFilterer) (*NodesStorageFilterer, error) {
	contract, err := bindNodesStorage(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &NodesStorageFilterer{contract: contract}, nil
}

// bindNodesStorage binds a generic wrapper to an already deployed contract.
func bindNodesStorage(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := NodesStorageMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_NodesStorage *NodesStorageRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _NodesStorage.Contract.NodesStorageCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_NodesStorage *NodesStorageRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NodesStorage.Contract.NodesStorageTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_NodesStorage *NodesStorageRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _NodesStorage.Contract.NodesStorageTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_NodesStorage *NodesStorageCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _NodesStorage.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_NodesStorage *NodesStorageTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NodesStorage.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_NodesStorage *NodesStorageTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _NodesStorage.Contract.contract.Transact(opts, method, params...)
}

// GetTotalValidNodes is a free data retrieval call binding the contract method 0x82fc2857.
//
// Solidity: function getTotalValidNodes() view returns(uint256 total)
func (_NodesStorage *NodesStorageCaller) GetTotalValidNodes(opts *bind.CallOpts) (*big.Int, error) {
	var out []interface{}
	err := _NodesStorage.contract.Call(opts, &out, "getTotalValidNodes")

	if err != nil {
		return *new(*big.Int), err
	}

	out0 := *abi.ConvertType(out[0], new(*big.Int)).(**big.Int)

	return out0, err

}

// GetTotalValidNodes is a free data retrieval call binding the contract method 0x82fc2857.
//
// Solidity: function getTotalValidNodes() view returns(uint256 total)
func (_NodesStorage *NodesStorageSession) GetTotalValidNodes() (*big.Int, error) {
	return _NodesStorage.Contract.GetTotalValidNodes(&_NodesStorage.CallOpts)
}

// GetTotalValidNodes is a free data retrieval call binding the contract method 0x82fc2857.
//
// Solidity: function getTotalValidNodes() view returns(uint256 total)
func (_NodesStorage *NodesStorageCallerSession) GetTotalValidNodes() (*big.Int, error) {
	return _NodesStorage.Contract.GetTotalValidNodes(&_NodesStorage.CallOpts)
}

// GetValidNodes is a free data retrieval call binding the contract method 0x11f1ba39.
//
// Solidity: function getValidNodes() view returns(address[] validNodes)
func (_NodesStorage *NodesStorageCaller) GetValidNodes(opts *bind.CallOpts) ([]common.Address, error) {
	var out []interface{}
	err := _NodesStorage.contract.Call(opts, &out, "getValidNodes")

	if err != nil {
		return *new([]common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new([]common.Address)).(*[]common.Address)

	return out0, err

}

// GetValidNodes is a free data retrieval call binding the contract method 0x11f1ba39.
//
// Solidity: function getValidNodes() view returns(address[] validNodes)
func (_NodesStorage *NodesStorageSession) GetValidNodes() ([]common.Address, error) {
	return _NodesStorage.Contract.GetValidNodes(&_NodesStorage.CallOpts)
}

// GetValidNodes is a free data retrieval call binding the contract method 0x11f1ba39.
//
// Solidity: function getValidNodes() view returns(address[] validNodes)
func (_NodesStorage *NodesStorageCallerSession) GetValidNodes() ([]common.Address, error) {
	return _NodesStorage.Contract.GetValidNodes(&_NodesStorage.CallOpts)
}

// IsValidNode is a free data retrieval call binding the contract method 0x9ebd11ef.
//
// Solidity: function isValidNode(address node) view returns(bool)
func (_NodesStorage *NodesStorageCaller) IsValidNode(opts *bind.CallOpts, node common.Address) (bool, error) {
	var out []interface{}
	err := _NodesStorage.contract.Call(opts, &out, "isValidNode", node)

	if err != nil {
		return *new(bool), err
	}

	out0 := *abi.ConvertType(out[0], new(bool)).(*bool)

	return out0, err

}

// IsValidNode is a free data retrieval call binding the contract method 0x9ebd11ef.
//
// Solidity: function isValidNode(address node) view returns(bool)
func (_NodesStorage *NodesStorageSession) IsValidNode(node common.Address) (bool, error) {
	return _NodesStorage.Contract.IsValidNode(&_NodesStorage.CallOpts, node)
}

// IsValidNode is a free data retrieval call binding the contract method 0x9ebd11ef.
//
// Solidity: function isValidNode(address node) view returns(bool)
func (_NodesStorage *NodesStorageCallerSession) IsValidNode(node common.Address) (bool, error) {
	return _NodesStorage.Contract.IsValidNode(&_NodesStorage.CallOpts, node)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_NodesStorage *NodesStorageCaller) Owner(opts *bind.CallOpts) (common.Address, error) {
	var out []interface{}
	err := _NodesStorage.contract.Call(opts, &out, "owner")

	if err != nil {
		return *new(common.Address), err
	}

	out0 := *abi.ConvertType(out[0], new(common.Address)).(*common.Address)

	return out0, err

}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_NodesStorage *NodesStorageSession) Owner() (common.Address, error) {
	return _NodesStorage.Contract.Owner(&_NodesStorage.CallOpts)
}

// Owner is a free data retrieval call binding the contract method 0x8da5cb5b.
//
// Solidity: function owner() view returns(address)
func (_NodesStorage *NodesStorageCallerSession) Owner() (common.Address, error) {
	return _NodesStorage.Contract.Owner(&_NodesStorage.CallOpts)
}

// AddNodes is a paid mutator transaction binding the contract method 0xdf9620eb.
//
// Solidity: function addNodes(address[] newNodes) returns()
func (_NodesStorage *NodesStorageTransactor) AddNodes(opts *bind.TransactOpts, newNodes []common.Address) (*types.Transaction, error) {
	return _NodesStorage.contract.Transact(opts, "addNodes", newNodes)
}

// AddNodes is a paid mutator transaction binding the contract method 0xdf9620eb.
//
// Solidity: function addNodes(address[] newNodes) returns()
func (_NodesStorage *NodesStorageSession) AddNodes(newNodes []common.Address) (*types.Transaction, error) {
	return _NodesStorage.Contract.AddNodes(&_NodesStorage.TransactOpts, newNodes)
}

// AddNodes is a paid mutator transaction binding the contract method 0xdf9620eb.
//
// Solidity: function addNodes(address[] newNodes) returns()
func (_NodesStorage *NodesStorageTransactorSession) AddNodes(newNodes []common.Address) (*types.Transaction, error) {
	return _NodesStorage.Contract.AddNodes(&_NodesStorage.TransactOpts, newNodes)
}

// RemoveNode is a paid mutator transaction binding the contract method 0xb2b99ec9.
//
// Solidity: function removeNode(address node) returns()
func (_NodesStorage *NodesStorageTransactor) RemoveNode(opts *bind.TransactOpts, node common.Address) (*types.Transaction, error) {
	return _NodesStorage.contract.Transact(opts, "removeNode", node)
}

// RemoveNode is a paid mutator transaction binding the contract method 0xb2b99ec9.
//
// Solidity: function removeNode(address node) returns()
func (_NodesStorage *NodesStorageSession) RemoveNode(node common.Address) (*types.Transaction, error) {
	return _NodesStorage.Contract.RemoveNode(&_NodesStorage.TransactOpts, node)
}

// RemoveNode is a paid mutator transaction binding the contract method 0xb2b99ec9.
//
// Solidity: function removeNode(address node) returns()
func (_NodesStorage *NodesStorageTransactorSession) RemoveNode(node common.Address) (*types.Transaction, error) {
	return _NodesStorage.Contract.RemoveNode(&_NodesStorage.TransactOpts, node)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_NodesStorage *NodesStorageTransactor) RenounceOwnership(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _NodesStorage.contract.Transact(opts, "renounceOwnership")
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_NodesStorage *NodesStorageSession) RenounceOwnership() (*types.Transaction, error) {
	return _NodesStorage.Contract.RenounceOwnership(&_NodesStorage.TransactOpts)
}

// RenounceOwnership is a paid mutator transaction binding the contract method 0x715018a6.
//
// Solidity: function renounceOwnership() returns()
func (_NodesStorage *NodesStorageTransactorSession) RenounceOwnership() (*types.Transaction, error) {
	return _NodesStorage.Contract.RenounceOwnership(&_NodesStorage.TransactOpts)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_NodesStorage *NodesStorageTransactor) TransferOwnership(opts *bind.TransactOpts, newOwner common.Address) (*types.Transaction, error) {
	return _NodesStorage.contract.Transact(opts, "transferOwnership", newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_NodesStorage *NodesStorageSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _NodesStorage.Contract.TransferOwnership(&_NodesStorage.TransactOpts, newOwner)
}

// TransferOwnership is a paid mutator transaction binding the contract method 0xf2fde38b.
//
// Solidity: function transferOwnership(address newOwner) returns()
func (_NodesStorage *NodesStorageTransactorSession) TransferOwnership(newOwner common.Address) (*types.Transaction, error) {
	return _NodesStorage.Contract.TransferOwnership(&_NodesStorage.TransactOpts, newOwner)
}

// NodesStorageNodeAddedIterator is returned from FilterNodeAdded and is used to iterate over the raw logs and unpacked data for NodeAdded events raised by the NodesStorage contract.
type NodesStorageNodeAddedIterator struct {
	Event *NodesStorageNodeAdded // Event containing the contract specifics and raw log

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
func (it *NodesStorageNodeAddedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NodesStorageNodeAdded)
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
		it.Event = new(NodesStorageNodeAdded)
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
func (it *NodesStorageNodeAddedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NodesStorageNodeAddedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NodesStorageNodeAdded represents a NodeAdded event raised by the NodesStorage contract.
type NodesStorageNodeAdded struct {
	Node common.Address
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterNodeAdded is a free log retrieval operation binding the contract event 0xb25d03aaf308d7291709be1ea28b800463cf3a9a4c4a5555d7333a964c1dfebd.
//
// Solidity: event NodeAdded(address indexed node)
func (_NodesStorage *NodesStorageFilterer) FilterNodeAdded(opts *bind.FilterOpts, node []common.Address) (*NodesStorageNodeAddedIterator, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _NodesStorage.contract.FilterLogs(opts, "NodeAdded", nodeRule)
	if err != nil {
		return nil, err
	}
	return &NodesStorageNodeAddedIterator{contract: _NodesStorage.contract, event: "NodeAdded", logs: logs, sub: sub}, nil
}

// WatchNodeAdded is a free log subscription operation binding the contract event 0xb25d03aaf308d7291709be1ea28b800463cf3a9a4c4a5555d7333a964c1dfebd.
//
// Solidity: event NodeAdded(address indexed node)
func (_NodesStorage *NodesStorageFilterer) WatchNodeAdded(opts *bind.WatchOpts, sink chan<- *NodesStorageNodeAdded, node []common.Address) (event.Subscription, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _NodesStorage.contract.WatchLogs(opts, "NodeAdded", nodeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NodesStorageNodeAdded)
				if err := _NodesStorage.contract.UnpackLog(event, "NodeAdded", log); err != nil {
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

// ParseNodeAdded is a log parse operation binding the contract event 0xb25d03aaf308d7291709be1ea28b800463cf3a9a4c4a5555d7333a964c1dfebd.
//
// Solidity: event NodeAdded(address indexed node)
func (_NodesStorage *NodesStorageFilterer) ParseNodeAdded(log types.Log) (*NodesStorageNodeAdded, error) {
	event := new(NodesStorageNodeAdded)
	if err := _NodesStorage.contract.UnpackLog(event, "NodeAdded", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NodesStorageNodeRemovedIterator is returned from FilterNodeRemoved and is used to iterate over the raw logs and unpacked data for NodeRemoved events raised by the NodesStorage contract.
type NodesStorageNodeRemovedIterator struct {
	Event *NodesStorageNodeRemoved // Event containing the contract specifics and raw log

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
func (it *NodesStorageNodeRemovedIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NodesStorageNodeRemoved)
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
		it.Event = new(NodesStorageNodeRemoved)
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
func (it *NodesStorageNodeRemovedIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NodesStorageNodeRemovedIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NodesStorageNodeRemoved represents a NodeRemoved event raised by the NodesStorage contract.
type NodesStorageNodeRemoved struct {
	Node common.Address
	Raw  types.Log // Blockchain specific contextual infos
}

// FilterNodeRemoved is a free log retrieval operation binding the contract event 0xcfc24166db4bb677e857cacabd1541fb2b30645021b27c5130419589b84db52b.
//
// Solidity: event NodeRemoved(address indexed node)
func (_NodesStorage *NodesStorageFilterer) FilterNodeRemoved(opts *bind.FilterOpts, node []common.Address) (*NodesStorageNodeRemovedIterator, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _NodesStorage.contract.FilterLogs(opts, "NodeRemoved", nodeRule)
	if err != nil {
		return nil, err
	}
	return &NodesStorageNodeRemovedIterator{contract: _NodesStorage.contract, event: "NodeRemoved", logs: logs, sub: sub}, nil
}

// WatchNodeRemoved is a free log subscription operation binding the contract event 0xcfc24166db4bb677e857cacabd1541fb2b30645021b27c5130419589b84db52b.
//
// Solidity: event NodeRemoved(address indexed node)
func (_NodesStorage *NodesStorageFilterer) WatchNodeRemoved(opts *bind.WatchOpts, sink chan<- *NodesStorageNodeRemoved, node []common.Address) (event.Subscription, error) {

	var nodeRule []interface{}
	for _, nodeItem := range node {
		nodeRule = append(nodeRule, nodeItem)
	}

	logs, sub, err := _NodesStorage.contract.WatchLogs(opts, "NodeRemoved", nodeRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NodesStorageNodeRemoved)
				if err := _NodesStorage.contract.UnpackLog(event, "NodeRemoved", log); err != nil {
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

// ParseNodeRemoved is a log parse operation binding the contract event 0xcfc24166db4bb677e857cacabd1541fb2b30645021b27c5130419589b84db52b.
//
// Solidity: event NodeRemoved(address indexed node)
func (_NodesStorage *NodesStorageFilterer) ParseNodeRemoved(log types.Log) (*NodesStorageNodeRemoved, error) {
	event := new(NodesStorageNodeRemoved)
	if err := _NodesStorage.contract.UnpackLog(event, "NodeRemoved", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}

// NodesStorageOwnershipTransferredIterator is returned from FilterOwnershipTransferred and is used to iterate over the raw logs and unpacked data for OwnershipTransferred events raised by the NodesStorage contract.
type NodesStorageOwnershipTransferredIterator struct {
	Event *NodesStorageOwnershipTransferred // Event containing the contract specifics and raw log

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
func (it *NodesStorageOwnershipTransferredIterator) Next() bool {
	// If the iterator failed, stop iterating
	if it.fail != nil {
		return false
	}
	// If the iterator completed, deliver directly whatever's available
	if it.done {
		select {
		case log := <-it.logs:
			it.Event = new(NodesStorageOwnershipTransferred)
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
		it.Event = new(NodesStorageOwnershipTransferred)
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
func (it *NodesStorageOwnershipTransferredIterator) Error() error {
	return it.fail
}

// Close terminates the iteration process, releasing any pending underlying
// resources.
func (it *NodesStorageOwnershipTransferredIterator) Close() error {
	it.sub.Unsubscribe()
	return nil
}

// NodesStorageOwnershipTransferred represents a OwnershipTransferred event raised by the NodesStorage contract.
type NodesStorageOwnershipTransferred struct {
	PreviousOwner common.Address
	NewOwner      common.Address
	Raw           types.Log // Blockchain specific contextual infos
}

// FilterOwnershipTransferred is a free log retrieval operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_NodesStorage *NodesStorageFilterer) FilterOwnershipTransferred(opts *bind.FilterOpts, previousOwner []common.Address, newOwner []common.Address) (*NodesStorageOwnershipTransferredIterator, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _NodesStorage.contract.FilterLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return &NodesStorageOwnershipTransferredIterator{contract: _NodesStorage.contract, event: "OwnershipTransferred", logs: logs, sub: sub}, nil
}

// WatchOwnershipTransferred is a free log subscription operation binding the contract event 0x8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e0.
//
// Solidity: event OwnershipTransferred(address indexed previousOwner, address indexed newOwner)
func (_NodesStorage *NodesStorageFilterer) WatchOwnershipTransferred(opts *bind.WatchOpts, sink chan<- *NodesStorageOwnershipTransferred, previousOwner []common.Address, newOwner []common.Address) (event.Subscription, error) {

	var previousOwnerRule []interface{}
	for _, previousOwnerItem := range previousOwner {
		previousOwnerRule = append(previousOwnerRule, previousOwnerItem)
	}
	var newOwnerRule []interface{}
	for _, newOwnerItem := range newOwner {
		newOwnerRule = append(newOwnerRule, newOwnerItem)
	}

	logs, sub, err := _NodesStorage.contract.WatchLogs(opts, "OwnershipTransferred", previousOwnerRule, newOwnerRule)
	if err != nil {
		return nil, err
	}
	return event.NewSubscription(func(quit <-chan struct{}) error {
		defer sub.Unsubscribe()
		for {
			select {
			case log := <-logs:
				// New log arrived, parse the event and forward to the user
				event := new(NodesStorageOwnershipTransferred)
				if err := _NodesStorage.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
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
func (_NodesStorage *NodesStorageFilterer) ParseOwnershipTransferred(log types.Log) (*NodesStorageOwnershipTransferred, error) {
	event := new(NodesStorageOwnershipTransferred)
	if err := _NodesStorage.contract.UnpackLog(event, "OwnershipTransferred", log); err != nil {
		return nil, err
	}
	event.Raw = log
	return event, nil
}
