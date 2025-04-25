package proto

import "google.golang.org/protobuf/proto"

func (m *IBFTMessage) PayloadNoSig() ([]byte, error) {
	mm, _ := proto.Clone(m).(*IBFTMessage)
	mm.Signature = nil

	raw, err := proto.Marshal(mm)
	if err != nil {
		return nil, err
	}

	return raw, nil
}
