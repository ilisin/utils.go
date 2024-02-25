package httpclient

import (
	"context"
	"net/http"
	"time"
)

type RequestHandler func(r *http.Request) error

type Options struct {
	timeout      time.Duration // request timeout
	baseURL      string        // base url
	encoder      Encoder       // request encoder
	decoder      Decoder       // response decoder
	handlers     []RequestHandler
	successCodes []int
}

func newOptions() *Options {
	return &Options{
		timeout:  DefaultTimeout,
		baseURL:  "",
		encoder:  defaultEncoder,
		decoder:  defaultDecoder,
		handlers: []RequestHandler{},
		successCodes: []int{
			http.StatusOK,
		},
	}
}

type Option func(*Options)

func WithTimeout(to time.Duration) Option { return func(o *Options) { o.timeout = to } }
func WithBaseURL(url string) Option       { return func(o *Options) { o.baseURL = url } }
func WithEncoder(e Encoder) Option        { return func(o *Options) { o.encoder = e } }
func WithDecoder(d Decoder) Option        { return func(o *Options) { o.decoder = d } }
func WithRequestHandler(h RequestHandler) Option {
	return func(o *Options) { o.handlers = append(o.handlers, h) }
}
func WithSuccessCodes(codes ...int) Option {
	return func(o *Options) { o.successCodes = append(o.successCodes, codes...) }
}

type contextKeyHeaders struct{}

func WithHeaders(ctx context.Context, headers map[string]string) context.Context {
	return context.WithValue(ctx, contextKeyHeaders{}, headers)
}

func GetHeaders(ctx context.Context) map[string]string {
	if headers, ok := ctx.Value(contextKeyHeaders{}).(map[string]string); ok {
		return headers
	}
	return nil
}
