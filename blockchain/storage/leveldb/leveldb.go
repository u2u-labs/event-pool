package leveldb

import (
	"fmt"

	"event-pool/blockchain/storage"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"go.uber.org/zap"
)

const DefaultOpenFileCap = 500

// Factory creates a leveldb storage
func Factory(config map[string]interface{}, logger *zap.SugaredLogger) (storage.Storage, error) {
	path, ok := config["path"]
	if !ok {
		return nil, fmt.Errorf("path not found")
	}

	pathStr, ok := path.(string)
	if !ok {
		return nil, fmt.Errorf("path is not a string")
	}

	return NewLevelDBStorage(pathStr, logger)
}

// NewLevelDBStorage creates the new storage reference with leveldb
func NewLevelDBStorage(path string, logger *zap.SugaredLogger) (storage.Storage, error) {
	ops := &opt.Options{
		OpenFilesCacheCapacity: DefaultOpenFileCap,
	}
	db, err := leveldb.OpenFile(path, ops)
	if err != nil {
		return nil, err
	}

	kv := &levelDBKV{db}

	return storage.NewKeyValueStorage(logger.Named("leveldb"), kv), nil
}

// levelDBKV is the leveldb implementation of the kv storage
type levelDBKV struct {
	db *leveldb.DB
}

// Set sets the key-value pair in leveldb storage
func (l *levelDBKV) Set(p []byte, v []byte) error {
	return l.db.Put(p, v, nil)
}

// Get retrieves the key-value pair in leveldb storage
func (l *levelDBKV) Get(p []byte) ([]byte, bool, error) {
	data, err := l.db.Get(p, nil)
	if err != nil {
		if err.Error() == "leveldb: not found" {
			return nil, false, nil
		}

		return nil, false, err
	}

	return data, true, nil
}

// Delete deletes the key in leveldb storage
func (l *levelDBKV) Delete(p []byte) bool {
	err := l.db.Delete(p, nil)
	if err != nil {
		if err.Error() == "leveldb: not found" {
			return false
		}

		return false
	}
	return true
}

// Close closes the leveldb storage instance
func (l *levelDBKV) Close() error {
	return l.db.Close()
}
