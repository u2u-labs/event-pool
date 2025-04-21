package prismadb

import (
	"context"
	"errors"

	"event-pool/blockchain/storage"
	"event-pool/helper/hex"
	"event-pool/prisma/db"
	"go.uber.org/zap"
)

// NewSQLStorage creates the new storage
func NewSQLStorage(logger *zap.SugaredLogger, client *db.PrismaClient) (storage.Storage, error) {
	sqlDb := &sqlDB{client}

	return storage.NewKeyValueStorage(logger, sqlDb), nil
}

// sqlDB is an in memory implementation of the kv storage
type sqlDB struct {
	client *db.PrismaClient
}

func (m *sqlDB) Set(p []byte, v []byte) error {
	ctx := context.Background()
	key := hex.EncodeToHex(p)

	// Try to update first
	_, err := m.client.KeyValue.UpsertOne(
		db.KeyValue.Key.Equals(key),
	).Update(
		db.KeyValue.Value.Set(v),
	).Create(
		db.KeyValue.Key.Set(key),
		db.KeyValue.Value.Set(v),
	).Exec(ctx)

	return err
}

func (m *sqlDB) Get(p []byte) ([]byte, bool, error) {
	ctx := context.Background()
	key := hex.EncodeToHex(p)

	record, err := m.client.KeyValue.FindUnique(
		db.KeyValue.Key.Equals(key),
	).Exec(ctx)

	if err != nil {
		// Check if it's a not found error
		if errors.Is(err, db.ErrNotFound) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return record.Value, true, nil
}

// TODO: skip bc prisma doesnt have delete method for obj??
func (m *sqlDB) Delete(p []byte) bool {
	//ctx := context.Background()
	//key := hex.EncodeToHex(p)

	//_, err := m.client.KeyValue.DeleteOne(
	//	db.KeyValue.Key.Equals(key),
	//).Exec(ctx)

	// If there's an error (like record doesn't exist), return false
	return true
}

func (m *sqlDB) Close() error {
	return m.client.Disconnect()
}
