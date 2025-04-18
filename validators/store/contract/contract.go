package contract

import (
	"errors"
	"fmt"
	"sync"

	"go.uber.org/zap"

	"event-pool/types"
	"event-pool/validators"
	"event-pool/validators/store"
	lru "github.com/hashicorp/golang-lru"
)

const (
	// How many validator sets are stored in the cache
	// Cache 3 validator sets for 3 epochs
	DefaultValidatorSetCacheSize = 3
)

var (
	ErrSignerNotFound                       = errors.New("signer not found")
	ErrInvalidValidatorsTypeAssertion       = errors.New("invalid type assertion for Validators")
	ErrInvalidValidatorsSubsetTypeAssertion = errors.New("invalid type assertion for Validators subset")
)

type ContractValidatorStore struct {
	sync.Mutex

	logger     *zap.SugaredLogger
	blockchain store.HeaderGetter
	executor   Executor

	// LRU cache for the validators
	validatorSetCache *lru.Cache
}

type Executor interface {
}

func NewContractValidatorStore(
	logger *zap.SugaredLogger,
	blockchain store.HeaderGetter,
	executor Executor,
	validatorSetCacheSize int,
) (*ContractValidatorStore, error) {
	var (
		validatorsCache *lru.Cache
		err             error
	)

	if validatorSetCacheSize > 0 {
		if validatorsCache, err = lru.New(validatorSetCacheSize); err != nil {
			return nil, fmt.Errorf("unable to create validator set cache, %w", err)
		}
	}

	return &ContractValidatorStore{
		logger:            logger,
		blockchain:        blockchain,
		executor:          executor,
		validatorSetCache: validatorsCache,
	}, nil
}

func (s *ContractValidatorStore) SourceType() store.SourceType {
	return store.Contract
}

// GetValidatorsByHeight get validators by querying the contract and save to cache, load from cache if found at height
func (s *ContractValidatorStore) GetValidatorsByHeight(
	validatorType validators.ValidatorType,
	height uint64,
) (validators.Validators, error) {
	cachedValidators, err := s.loadCachedValidatorSet(height)
	if err != nil {
		return nil, err
	}

	if cachedValidators != nil {
		return cachedValidators, nil
	}

	fetchedValidators, err := FetchValidators(validatorType, types.ZeroAddress)
	if err != nil {
		return nil, err
	}

	s.saveToValidatorSetCache(height, fetchedValidators)

	return fetchedValidators, nil
}

// loadCachedValidatorSet loads validators from validatorSetCache
func (s *ContractValidatorStore) loadCachedValidatorSet(height uint64) (validators.Validators, error) {
	cachedRawValidators, ok := s.validatorSetCache.Get(height)
	if !ok {
		return nil, nil
	}

	_validators, ok := cachedRawValidators.(validators.Validators)
	if !ok {
		return nil, ErrInvalidValidatorsTypeAssertion
	}

	return _validators, nil
}

// saveToValidatorSetCache saves validators to validatorSetCache
func (s *ContractValidatorStore) saveToValidatorSetCache(height uint64, validators validators.Validators) bool {
	if s.validatorSetCache == nil {
		return false
	}

	return s.validatorSetCache.Add(height, validators)
}

// ModifyHeader updates Header Miner address
func (s *ContractValidatorStore) ModifyHeader(header *types.Header, proposer types.Address) error {
	header.Creator = proposer

	return nil
}

// VerifyHeader empty implementation
func (s *ContractValidatorStore) VerifyHeader(*types.Header) error {
	return nil
}

// ProcessHeader empty implementation
func (s *ContractValidatorStore) ProcessHeader(*types.Header) error {
	return nil
}
