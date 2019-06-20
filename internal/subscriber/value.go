package subscriber

import "encoding/json"

type payloadValue struct {
	Value json.RawMessage `json:"value"`
}
