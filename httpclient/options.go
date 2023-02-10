package httpclient

import (
	"net/http"
	"time"
)

type Option interface {
	SetOption(client *HttpClient)
}

type RequestHandler func(r *http.Request) error

type OptionTimeout struct{ timeout time.Duration }

func NewTimeoutOption(to time.Duration) Option         { return &OptionTimeout{timeout: to} }
func (ot *OptionTimeout) SetOption(client *HttpClient) { client.c.Timeout = ot.timeout }

type OptionBaseURL struct{ baseURL string }

func NewBaseURLOption(url string) Option               { return &OptionBaseURL{baseURL: url} }
func (ob *OptionBaseURL) SetOption(client *HttpClient) { client.baseURL = ob.baseURL }

type OptionLog struct{ log bool }

func NewLogOption(l bool) Option                   { return &OptionLog{log: l} }
func (lo *OptionLog) SetOption(client *HttpClient) { client.log = lo.log }

type OptionEncoder struct{ encoder Encoder }

func NewEncoderOption(e Encoder) Option                { return &OptionEncoder{encoder: e} }
func (oe *OptionEncoder) SetOption(client *HttpClient) { client.requestEncoder = oe.encoder }

type OptionDecoder struct{ decoder Decoder }

func NewDecoderOption(d Decoder) Option                { return &OptionDecoder{decoder: d} }
func (od *OptionDecoder) SetOption(client *HttpClient) { client.responseDecoder = od.decoder }
