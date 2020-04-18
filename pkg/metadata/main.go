package metadata

import (
	"bytes"
	"encoding/json"
	"io"
)

// Metadata defines a JSON structure holding annotations about an object.
type Metadata map[string]interface{}

func NewFromReader(reader io.Reader) (*Metadata, error) {
	meta := &Metadata{}
	err := json.NewDecoder(reader).Decode(meta)
	if err != nil {
		return nil, err
	}
	return meta, nil

}

func (meta *Metadata) Set(key string, value interface{}) {
	// If incoming value is a valid JSON string, store it as JSON, not a string.
	if valueAsString, ok := value.(string); ok {
		jsonValue := &json.RawMessage{}
		if err := json.Unmarshal([]byte(valueAsString), &jsonValue); err == nil {
			(*meta)[key] = jsonValue
			return
		}
	}
	// Store value in exact form it arrived.
	(*meta)[key] = value
}

func (meta *Metadata) Delete(key string) {
	delete(*meta, key)
}

func (meta *Metadata) ToReader() (io.Reader, error) {
	jsonBytes, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(jsonBytes), nil
}
