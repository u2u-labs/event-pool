package contract

import (
	"fmt"

	//"event-pool/contracts/staking"
	"event-pool/types"
	"event-pool/validators"
)

// FetchValidators fetches validators from a contract switched by validator type
func FetchValidators(
	validatorType validators.ValidatorType,
	from types.Address,
) (validators.Validators, error) {
	switch validatorType {
	case validators.ECDSAValidatorType:
		return FetchECDSAValidators(from)
	}

	return nil, fmt.Errorf("unsupported validator type: %s", validatorType)
}

// FetchECDSAValidators queries a contract for validator addresses and returns ECDSAValidators
func FetchECDSAValidators(
	from types.Address,
) (validators.Validators, error) {
	//valAddrs, err := staking.QueryValidators(from)
	//if err != nil {
	//	return nil, err
	//}

	ecdsaValidators := validators.NewECDSAValidatorSet()
	//for _, addr := range valAddrs {
	//	if err := ecdsaValidators.Add(validators.NewECDSAValidator(addr)); err != nil {
	//		return nil, err
	//	}
	//}

	return ecdsaValidators, nil
}
