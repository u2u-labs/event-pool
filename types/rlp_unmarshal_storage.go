package types

type RLPStoreUnmarshaler interface {
	UnmarshalStoreRLP(input []byte) error
}
