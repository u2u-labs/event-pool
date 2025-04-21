package memory

import (
	"event-pool/blockchain/storage"
	"event-pool/helper/hex"
	"go.uber.org/zap"
)

// NewMemoryStorage creates the new storage reference with inmemory
func NewMemoryStorage(logger *zap.SugaredLogger) (storage.Storage, error) {
	db := &memoryKV{map[string][]byte{}}

	return storage.NewKeyValueStorage(logger, db), nil
}

// memoryKV is an in memory implementation of the kv storage
type memoryKV struct {
	db map[string][]byte
}

func (m *memoryKV) Set(p []byte, v []byte) error {
	m.db[hex.EncodeToHex(p)] = v

	return nil
}

func (m *memoryKV) Get(p []byte) ([]byte, bool, error) {
	v, ok := m.db[hex.EncodeToHex(p)]
	if !ok {
		return nil, false, nil
	}

	return v, true, nil
}

func (m *memoryKV) Delete(p []byte) bool {
	//m.db[hex.EncodeToHex(p)] = nil
	delete(m.db, hex.EncodeToHex(p))

	return true
}

func (m *memoryKV) Close() error {
	return nil
}
