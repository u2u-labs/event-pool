package grpc

import "encoding/json"

func Data(data any) []byte {
	dataResp := map[string]any{
		"data": data,
	}
	resp, err := json.Marshal(dataResp)
	if err != nil {
		return nil
	}
	return resp
}
