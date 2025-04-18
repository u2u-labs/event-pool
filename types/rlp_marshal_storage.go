package types

type RLPStoreMarshaler interface {
	MarshalStoreRLPTo(dst []byte) []byte
}
