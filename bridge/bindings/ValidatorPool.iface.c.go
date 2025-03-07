// Generated by ifacemaker. DO NOT EDIT.

package bindings

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

// IValidatorPoolCaller ...
type IValidatorPoolCaller interface {
	// CLAIMPERIOD is a free data retrieval call binding the contract method 0x21241dfe.
	//
	// Solidity: function CLAIM_PERIOD() view returns(uint256)
	CLAIMPERIOD(opts *bind.CallOpts) (*big.Int, error)
	// POSITIONLOCKPERIOD is a free data retrieval call binding the contract method 0x9c87e3ed.
	//
	// Solidity: function POSITION_LOCK_PERIOD() view returns(uint256)
	POSITIONLOCKPERIOD(opts *bind.CallOpts) (*big.Int, error)
	// GetDisputerReward is a free data retrieval call binding the contract method 0x9ccdf830.
	//
	// Solidity: function getDisputerReward() view returns(uint256)
	GetDisputerReward(opts *bind.CallOpts) (*big.Int, error)
	// GetLocation is a free data retrieval call binding the contract method 0xd9e0dc59.
	//
	// Solidity: function getLocation(address validator_) view returns(string)
	GetLocation(opts *bind.CallOpts, validator_ common.Address) (string, error)
	// GetLocations is a free data retrieval call binding the contract method 0x76207f9c.
	//
	// Solidity: function getLocations(address[] validators_) view returns(string[])
	GetLocations(opts *bind.CallOpts, validators_ []common.Address) ([]string, error)
	// GetMaxIntervalWithoutSnapshots is a free data retrieval call binding the contract method 0xd3dfe445.
	//
	// Solidity: function getMaxIntervalWithoutSnapshots() view returns(uint256 maxIntervalWithoutSnapshots)
	GetMaxIntervalWithoutSnapshots(opts *bind.CallOpts) (*big.Int, error)
	// GetMaxNumValidators is a free data retrieval call binding the contract method 0xd2992f54.
	//
	// Solidity: function getMaxNumValidators() view returns(uint256)
	GetMaxNumValidators(opts *bind.CallOpts) (*big.Int, error)
	// GetMetamorphicContractAddress is a free data retrieval call binding the contract method 0x8653a465.
	//
	// Solidity: function getMetamorphicContractAddress(bytes32 _salt, address _factory) pure returns(address)
	GetMetamorphicContractAddress(opts *bind.CallOpts, _salt [32]byte, _factory common.Address) (common.Address, error)
	// GetStakeAmount is a free data retrieval call binding the contract method 0x722580b6.
	//
	// Solidity: function getStakeAmount() view returns(uint256)
	GetStakeAmount(opts *bind.CallOpts) (*big.Int, error)
	// GetValidator is a free data retrieval call binding the contract method 0xb5d89627.
	//
	// Solidity: function getValidator(uint256 index_) view returns(address)
	GetValidator(opts *bind.CallOpts, index_ *big.Int) (common.Address, error)
	// GetValidatorData is a free data retrieval call binding the contract method 0xc0951451.
	//
	// Solidity: function getValidatorData(uint256 index_) view returns((address,uint256))
	GetValidatorData(opts *bind.CallOpts, index_ *big.Int) (ValidatorData, error)
	// GetValidatorsAddresses is a free data retrieval call binding the contract method 0x9c7d8961.
	//
	// Solidity: function getValidatorsAddresses() view returns(address[])
	GetValidatorsAddresses(opts *bind.CallOpts) ([]common.Address, error)
	// GetValidatorsCount is a free data retrieval call binding the contract method 0x27498240.
	//
	// Solidity: function getValidatorsCount() view returns(uint256)
	GetValidatorsCount(opts *bind.CallOpts) (*big.Int, error)
	// IsAccusable is a free data retrieval call binding the contract method 0x20c2856d.
	//
	// Solidity: function isAccusable(address account_) view returns(bool)
	IsAccusable(opts *bind.CallOpts, account_ common.Address) (bool, error)
	// IsConsensusRunning is a free data retrieval call binding the contract method 0xc8d1a5e4.
	//
	// Solidity: function isConsensusRunning() view returns(bool)
	IsConsensusRunning(opts *bind.CallOpts) (bool, error)
	// IsInExitingQueue is a free data retrieval call binding the contract method 0xe4ad75f1.
	//
	// Solidity: function isInExitingQueue(address account_) view returns(bool)
	IsInExitingQueue(opts *bind.CallOpts, account_ common.Address) (bool, error)
	// IsMaintenanceScheduled is a free data retrieval call binding the contract method 0x1885570f.
	//
	// Solidity: function isMaintenanceScheduled() view returns(bool)
	IsMaintenanceScheduled(opts *bind.CallOpts) (bool, error)
	// IsValidator is a free data retrieval call binding the contract method 0xfacd743b.
	//
	// Solidity: function isValidator(address account_) view returns(bool)
	IsValidator(opts *bind.CallOpts, account_ common.Address) (bool, error)
	// TryGetTokenID is a free data retrieval call binding the contract method 0xee9e49bd.
	//
	// Solidity: function tryGetTokenID(address account_) view returns(bool, address, uint256)
	TryGetTokenID(opts *bind.CallOpts, account_ common.Address) (bool, common.Address, *big.Int, error)
}
