package fork

import (
	"event-pool/consensus/ibft/signer"
	"event-pool/validators"
	"event-pool/validators/store"
	"event-pool/validators/store/contract"
	"go.uber.org/zap"
)

// ContractValidatorStoreWrapper is a wrapper of *contract.ContractValidatorStore
// in order to add Close and GetValidators
type ContractValidatorStoreWrapper struct {
	*contract.ContractValidatorStore
	getSigner func(uint64) (signer.Signer, error)
}

// NewContractValidatorStoreWrapper creates *ContractValidatorStoreWrapper
func NewContractValidatorStoreWrapper(
	logger *zap.SugaredLogger,
	blockchain store.HeaderGetter,
	getSigner func(uint64) (signer.Signer, error),
) (*ContractValidatorStoreWrapper, error) {
	contractStore, err := contract.NewContractValidatorStore(
		logger,
		blockchain,
		contract.DefaultValidatorSetCacheSize,
	)

	if err != nil {
		return nil, err
	}

	return &ContractValidatorStoreWrapper{
		ContractValidatorStore: contractStore,
		getSigner:              getSigner,
	}, nil
}

// Close is closer process
func (w *ContractValidatorStoreWrapper) Close() error {
	return nil
}

// GetValidators gets and returns validators at the given height
func (w *ContractValidatorStoreWrapper) GetValidators(
	height, epochSize, forkFrom uint64,
) (validators.Validators, error) {
	signer, err := w.getSigner(height)
	if err != nil {
		return nil, err
	}

	return w.GetValidatorsByHeight(
		signer.Type(),
		calculateContractStoreFetchingHeight(
			height,
			epochSize,
			forkFrom,
		),
	)
}

// calculateContractStoreFetchingHeight calculates the block height at which ContractStore fetches validators
// based on height, epoch, and fork beginning height
func calculateContractStoreFetchingHeight(height, epochSize, forkFrom uint64) uint64 {
	// calculates the beginning of the epoch the given height is in
	beginningEpoch := (height / epochSize) * epochSize

	// calculates the end of the previous epoch
	// to determine the height to fetch validators
	fetchingHeight := uint64(0)
	if beginningEpoch > 0 {
		fetchingHeight = beginningEpoch - 1
	}

	// use the calculated height if it's bigger than or equal to from
	if fetchingHeight >= forkFrom {
		return fetchingHeight
	}

	if forkFrom > 0 {
		return forkFrom - 1
	}

	return forkFrom
}
