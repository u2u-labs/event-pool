package validators

import (
	"errors"
	"fmt"

	"event-pool/types"
)

var (
	ErrInvalidBLSValidatorFormat = errors.New("invalid validator format, expected [Validator Address]:[BLS Public Key]")
)

// NewValidatorFromType instantiates a validator by specified type
func NewValidatorFromType(t ValidatorType) (Validator, error) {
	switch t {
	case ECDSAValidatorType:
		return new(ECDSAValidator), nil
	}

	return nil, ErrInvalidValidatorType
}

// NewValidatorSetFromType instantiates a validators by specified type
func NewValidatorSetFromType(t ValidatorType) Validators {
	switch t {
	case ECDSAValidatorType:
		return NewECDSAValidatorSet()
	}

	return nil
}

// NewECDSAValidatorSet creates Validator Set for ECDSAValidator with initialized validators
func NewECDSAValidatorSet(ecdsaValidators ...*ECDSAValidator) Validators {
	validators := make([]Validator, len(ecdsaValidators))

	for idx, val := range ecdsaValidators {
		validators[idx] = Validator(val)
	}

	return &Set{
		ValidatorType: ECDSAValidatorType,
		Validators:    validators,
	}
}

// ParseValidator parses a validator represented in string
func ParseValidator(validatorType ValidatorType, validator string) (Validator, error) {
	switch validatorType {
	case ECDSAValidatorType:
		return ParseECDSAValidator(validator), nil
	default:
		// shouldn't reach here
		return nil, fmt.Errorf("invalid validator type: %s", validatorType)
	}
}

// ParseValidator parses an array of validator represented in string
func ParseValidators(validatorType ValidatorType, rawValidators []string) (Validators, error) {
	set := NewValidatorSetFromType(validatorType)
	if set == nil {
		return nil, fmt.Errorf("invalid validator type: %s", validatorType)
	}

	for _, s := range rawValidators {
		validator, err := ParseValidator(validatorType, s)
		if err != nil {
			return nil, err
		}

		if err := set.Add(validator); err != nil {
			return nil, err
		}
	}

	return set, nil
}

// ParseBLSValidator parses ECDSAValidator represented in string
func ParseECDSAValidator(validator string) *ECDSAValidator {
	return &ECDSAValidator{
		Address: types.StringToAddress(validator),
	}
}
