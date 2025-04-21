package contract

import (
	"fmt"

	nodestorage "event-pool/contracts/nodesstorage"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"

	//"event-pool/contracts/staking"
	"event-pool/types"
	"event-pool/validators"
)

// FetchValidators fetches validators from a contract switched by validator type
func FetchValidators(
	validatorType validators.ValidatorType,
	from types.Address,
	to types.Address,
	client bind.ContractBackend,
) (validators.Validators, error) {
	switch validatorType {
	case validators.ECDSAValidatorType:
		return FetchECDSAValidators(from, to, client)
	}

	return nil, fmt.Errorf("unsupported validator type: %s", validatorType)
}

// FetchECDSAValidators queries a contract for validator addresses and returns ECDSAValidators
func FetchECDSAValidators(
	from types.Address,
	to types.Address,
	client bind.ContractBackend,
) (validators.Validators, error) {
	ns, err := nodestorage.NewNodesStorage(common.Address(to), client)
	if err != nil {
		return nil, err
	}
	valAddrs, err := ns.GetValidNodes(&bind.CallOpts{
		From: common.Address(from),
	})
	if err != nil {
		return nil, err
	}

	ecdsaValidators := validators.NewECDSAValidatorSet()
	for _, addr := range valAddrs {
		if err := ecdsaValidators.Add(validators.NewECDSAValidator(types.Address(addr))); err != nil {
			return nil, err
		}
	}

	return ecdsaValidators, nil
}
