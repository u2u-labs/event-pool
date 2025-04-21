package fork

import (
	"errors"
	"fmt"
	"testing"

	"event-pool/blockchain"
	"event-pool/consensus/ibft/signer"
	"event-pool/network/common"
	"github.com/stretchr/testify/assert"
)

var (
	errTest = errors.New("test")
)

func createTestMetadataJSON(height uint64) string {
	return fmt.Sprintf(`{"LastBlock": %d}`, height)
}

func TestNewContractValidatorStoreWrapper(t *testing.T) {
	t.Parallel()

	_, err := NewContractValidatorStoreWrapper(
		common.NewNullSugaredLogger(),
		&blockchain.Blockchain{},
		func(u uint64) (signer.Signer, error) {
			return nil, nil
		},
	)

	assert.NoError(t, err)
}

func TestNewContractValidatorStoreWrapperClose(t *testing.T) {
	t.Parallel()

	wrapper, err := NewContractValidatorStoreWrapper(
		common.NewNullSugaredLogger(),
		&blockchain.Blockchain{},
		func(u uint64) (signer.Signer, error) {
			return nil, nil
		},
	)

	assert.NoError(t, err)
	assert.NoError(t, wrapper.Close())
}

func TestNewContractValidatorStoreWrapperGetValidators(t *testing.T) {
	t.Parallel()

	t.Run("should return error if getSigner returns error", func(t *testing.T) {
		t.Parallel()

		wrapper, err := NewContractValidatorStoreWrapper(
			common.NewNullSugaredLogger(),
			&blockchain.Blockchain{},
			func(u uint64) (signer.Signer, error) {
				return nil, errTest
			},
		)

		assert.NoError(t, err)

		res, err := wrapper.GetValidators(0, 0, 0)
		assert.Nil(t, res)
		assert.ErrorIs(t, errTest, err)
	})

	t.Run("should return error if GetValidatorsByHeight returns error", func(t *testing.T) {
		t.Parallel()
		t.Skip()

		wrapper, err := NewContractValidatorStoreWrapper(
			common.NewNullSugaredLogger(),
			&blockchain.Blockchain{},
			func(u uint64) (signer.Signer, error) {
				return signer.NewSigner(
					&signer.ECDSAKeyManager{},
					nil,
				), nil
			},
		)

		assert.NoError(t, err)

		res, err := wrapper.GetValidators(10, 10, 0)
		assert.Nil(t, res)
		assert.ErrorContains(t, err, "header not found at 9")
	})
}

func Test_calculateContractStoreFetchingHeight(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		height    uint64
		epochSize uint64
		forkFrom  uint64
		expected  uint64
	}{
		{
			name:      "should return 0 if the height is 2 (in the first epoch)",
			height:    2,
			epochSize: 10,
			forkFrom:  0,
			expected:  0,
		},
		{
			name:      "should return 0 if the height is 9 (in the first epoch)",
			height:    9,
			epochSize: 10,
			forkFrom:  0,
			expected:  0,
		},
		{
			name:      "should return 9 if the height is 10 (in the second epoch)",
			height:    10,
			epochSize: 10,
			forkFrom:  0,
			expected:  9,
		},
		{
			name:      "should return 9 if the height is 19 (in the second epoch)",
			height:    19,
			epochSize: 10,
			forkFrom:  0,
			expected:  9,
		},
		{
			name:      "should return 49 if the height is 10 but forkFrom is 50",
			height:    10,
			epochSize: 10,
			forkFrom:  50,
			expected:  49,
		},
		{
			name:      "should return 59 if the height is 60 and forkFrom is 50",
			height:    60,
			epochSize: 10,
			forkFrom:  50,
			expected:  59,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(
				t,
				test.expected,
				calculateContractStoreFetchingHeight(test.height, test.epochSize, test.forkFrom),
			)
		})
	}
}
