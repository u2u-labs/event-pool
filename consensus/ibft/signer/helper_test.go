package signer

import (
	"crypto/ecdsa"
	"errors"
	"testing"

	"event-pool/crypto"
	"event-pool/helper/hex"
	"event-pool/secrets"
	"event-pool/types"
	"event-pool/validators"
	"github.com/stretchr/testify/assert"
)

var (
	testHeader = &types.Header{
		ParentHash: types.BytesToHash(crypto.Keccak256([]byte{0x1})),
		Number:     9,
		Timestamp:  12,
		ExtraData:  crypto.Keccak256([]byte{0x13}),
	}

	testHeaderHashHex = "0xd72807308cdfcb9c5a214ae044607a89498130a881fa97c643b9aed54fe209bb"
)

func newTestECDSAKey(t *testing.T) (*ecdsa.PrivateKey, []byte) {
	t.Helper()

	testKey, testKeyEncoded, err := crypto.GenerateAndEncodeECDSAPrivateKey()
	assert.NoError(t, err, "failed to initialize ECDSA key")

	return testKey, testKeyEncoded
}

// Make sure the target function always returns the same result
func Test_wrapCommitHash(t *testing.T) {
	t.Parallel()

	var (
		input             = crypto.Keccak256([]byte{0x1})
		expectedOutputHex = "0x8a319084d2e52be9c9192645aa98900413ee2a7c93c2916ef99d62218207d1da"
	)

	expectedOutput, err := hex.DecodeHex(expectedOutputHex)
	if err != nil {
		t.Fatalf("failed to parse expected output: %s, %v", expectedOutputHex, err)
	}

	output := wrapCommitHash(input)

	assert.Equal(t, expectedOutput, output)
}

// nolint
func Test_getOrCreateECDSAKey(t *testing.T) {
	t.Parallel()

	testKey, testKeyEncoded := newTestECDSAKey(t)

	testSecretName := func(name string) {
		t.Helper()

		// make sure that the correct key is given
		assert.Equal(t, secrets.ValidatorKey, name)
	}

	//lint:ignore dupl
	tests := []struct {
		name              string
		mockSecretManager *MockSecretManager
		expectedResult    *ecdsa.PrivateKey
		expectedErr       error
	}{
		{
			name: "should load ECDSA key from secret manager if the key exists",
			mockSecretManager: &MockSecretManager{
				HasSecretFn: func(name string) bool {
					testSecretName(name)

					return true
				},
				GetSecretFn: func(name string) ([]byte, error) {
					testSecretName(name)

					return testKeyEncoded, nil
				},
			},
			expectedResult: testKey,
			expectedErr:    nil,
		},
		{
			name: "should create new ECDSA key if the key doesn't exist",
			mockSecretManager: &MockSecretManager{
				HasSecretFn: func(name string) bool {
					testSecretName(name)

					return false
				},
				SetSecretFn: func(name string, key []byte) error {
					testSecretName(name)

					assert.NotEqual(t, testKeyEncoded, key)

					return nil
				},
				GetSecretFn: func(name string) ([]byte, error) {
					testSecretName(name)

					return testKeyEncoded, nil
				},
			},
			expectedResult: testKey,
			expectedErr:    nil,
		},
		{
			name: "should return error if secret manager returns error",
			mockSecretManager: &MockSecretManager{
				HasSecretFn: func(name string) bool {
					testSecretName(name)

					return true
				},
				GetSecretFn: func(name string) ([]byte, error) {
					testSecretName(name)

					return nil, errTest
				},
			},
			expectedResult: nil,
			expectedErr:    errTest,
		},
		{
			name: "should return error if the key manager fails to generate new ECDSA key",
			mockSecretManager: &MockSecretManager{
				HasSecretFn: func(name string) bool {
					testSecretName(name)

					return false
				},
				SetSecretFn: func(name string, key []byte) error {
					testSecretName(name)

					return errTest
				},
			},
			expectedResult: nil,
			expectedErr:    errTest,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			res, err := getOrCreateECDSAKey(test.mockSecretManager)

			assert.Equal(t, test.expectedResult, res)
			assert.ErrorIs(t, test.expectedErr, err)
		})
	}
}

// make sure that header hash calculation returns the same hash
func Test_calculateHeaderHash(t *testing.T) {
	t.Parallel()

	t.Logf("%v", calculateHeaderHash(testHeader))
	assert.Equal(
		t,
		types.StringToHash(testHeaderHashHex),
		calculateHeaderHash(testHeader),
	)
}

func Test_ecrecover(t *testing.T) {
	t.Parallel()

	testKey, _ := newTestECDSAKey(t)
	signerAddress := crypto.PubKeyToAddress(&testKey.PublicKey)

	rawMessage := crypto.Keccak256([]byte{0x1})

	signature, err := crypto.Sign(
		testKey,
		rawMessage,
	)
	assert.NoError(t, err)

	recoveredAddress, err := ecrecover(signature, rawMessage)
	assert.NoError(t, err)

	assert.Equal(
		t,
		signerAddress,
		recoveredAddress,
	)
}

func TestNewKeyManagerFromType(t *testing.T) {
	t.Parallel()

	testECDSAKey, testECDSAKeyEncoded := newTestECDSAKey(t)

	tests := []struct {
		name              string
		validatorType     validators.ValidatorType
		mockSecretManager *MockSecretManager
		expectedRes       KeyManager
		expectedErr       error
	}{
		{
			name:          "ECDSAValidatorType",
			validatorType: validators.ECDSAValidatorType,
			mockSecretManager: &MockSecretManager{
				HasSecretFn: func(name string) bool {
					return true
				},
				GetSecretFn: func(name string) ([]byte, error) {
					return testECDSAKeyEncoded, nil
				},
			},
			expectedRes: NewECDSAKeyManagerFromKey(testECDSAKey),
			expectedErr: nil,
		},
		{
			name:          "unsupported type",
			validatorType: validators.ValidatorType("fake"),
			expectedRes:   nil,
			expectedErr:   errors.New("unsupported validator type: fake"),
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			res, err := NewKeyManagerFromType(test.mockSecretManager, test.validatorType)

			assert.Equal(t, test.expectedRes, res)

			if test.expectedErr == nil {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.ErrorContains(t, err, test.expectedErr.Error())
			}
		})
	}
}

func Test_verifyIBFTExtraSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		extraData []byte
		isError   bool
	}{
		{
			name:      "should return error if ExtraData size is 0",
			extraData: make([]byte, 0),
			isError:   true,
		},
		{
			name:      "should return error if ExtraData size is less than IstanbulExtraVanity",
			extraData: make([]byte, IstanbulExtraVanity-1),
			isError:   true,
		},
		{
			name:      "should return nil if ExtraData size matches with IstanbulExtraVanity",
			extraData: make([]byte, IstanbulExtraVanity),
			isError:   false,
		},
		{
			name:      "should return nil if ExtraData size is greater than IstanbulExtraVanity",
			extraData: make([]byte, IstanbulExtraVanity+1),
			isError:   false,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			header := &types.Header{
				ExtraData: test.extraData,
			}

			err := verifyIBFTExtraSize(header)

			if test.isError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
