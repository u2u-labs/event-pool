package validators

import (
	"encoding/hex"
	"fmt"
	"testing"

	"event-pool/types"
	"github.com/stretchr/testify/assert"
)

var (
	addr1 = types.StringToAddress("1")
	addr2 = types.StringToAddress("2")

	ecdsaValidator1 = NewECDSAValidator(addr1)
	ecdsaValidator2 = NewECDSAValidator(addr2)

	fakeValidatorType = ValidatorType("fake")
)

func createTestBLSValidatorString(
	addr types.Address,
	blsPubKey []byte,
) string {
	return fmt.Sprintf(
		"%s:%s",
		addr.String(),
		"0x"+hex.EncodeToString(blsPubKey),
	)
}

func TestNewValidatorFromType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		validatorType ValidatorType
		expected      Validator
		err           error
	}{
		{
			name:          "ECDSAValidator",
			validatorType: ECDSAValidatorType,
			expected:      new(ECDSAValidator),
			err:           nil,
		},
		{
			name:          "undefined type",
			validatorType: fakeValidatorType,
			expected:      nil,
			err:           ErrInvalidValidatorType,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			res, err := NewValidatorFromType(test.validatorType)

			assert.Equal(
				t,
				test.expected,
				res,
			)

			assert.ErrorIs(
				t,
				test.err,
				err,
			)
		})
	}
}

func TestNewValidatorSetFromType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		validatorType ValidatorType
		expected      Validators
	}{
		{
			name:          "ECDSAValidators",
			validatorType: ECDSAValidatorType,
			expected: &Set{
				ValidatorType: ECDSAValidatorType,
				Validators:    []Validator{},
			},
		},
		{
			name:          "undefined type",
			validatorType: fakeValidatorType,
			expected:      nil,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(
				t,
				test.expected,
				NewValidatorSetFromType(test.validatorType),
			)
		})
	}
}

func TestParseValidator(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// inputs
		validatorType ValidatorType
		validatorStr  string
		// outputs
		expectedValidator Validator
		expectedErr       error
	}{
		{
			name:              "ECDSAValidator",
			validatorType:     ECDSAValidatorType,
			validatorStr:      addr1.String(),
			expectedValidator: ecdsaValidator1,
			expectedErr:       nil,
		},
		{
			name:              "undefined type",
			validatorType:     fakeValidatorType,
			validatorStr:      addr1.String(),
			expectedValidator: nil,
			expectedErr:       fmt.Errorf("invalid validator type: %s", fakeValidatorType),
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			val, err := ParseValidator(
				test.validatorType,
				test.validatorStr,
			)

			assert.Equal(t, test.expectedValidator, val)

			assert.Equal(t, test.expectedErr, err)
		})
	}
}

func TestParseValidators(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		// inputs
		validatorType ValidatorType
		validatorStrs []string
		// outputs
		expectedValidators Validators
		expectedErr        error
	}{
		{
			name:          "ECDSAValidator",
			validatorType: ECDSAValidatorType,
			validatorStrs: []string{
				addr1.String(),
				addr2.String(),
			},
			expectedValidators: NewECDSAValidatorSet(
				ecdsaValidator1,
				ecdsaValidator2,
			),
			expectedErr: nil,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			vals, err := ParseValidators(
				test.validatorType,
				test.validatorStrs,
			)

			assert.Equal(t, test.expectedValidators, vals)

			assert.Equal(t, test.expectedErr, err)
		})
	}
}

func TestParseECDSAValidator(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t,
		ecdsaValidator1,
		ParseECDSAValidator(addr1.String()),
	)
}
