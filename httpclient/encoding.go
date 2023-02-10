package httpclient

import (
	"encoding/json"
)

type Decoder interface {
	Decode(data []byte, v interface{}) error
}

type Encoder interface {
	Encode(v interface{}) ([]byte, error)
}

type JsonDecoder struct{}

func (*JsonDecoder) Decode(data []byte, v interface{}) error { return json.Unmarshal(data, v) }

type JsonEncoder struct{}

func (*JsonEncoder) Encode(v interface{}) ([]byte, error) { return json.Marshal(v) }

var (
	jsonDecoder = new(JsonDecoder)
	jsonEncoder = new(JsonEncoder)

	defaultDecoder = jsonDecoder
	defaultEncoder = jsonEncoder
)
