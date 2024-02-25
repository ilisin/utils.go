package httpclient

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/samber/lo"
)

const DefaultTimeout = 5 * time.Second

type HttpClient struct {
	options *Options

	c *http.Client
}

func New(opts ...Option) *HttpClient {
	hc := new(HttpClient)
	hc.options = newOptions()
	for _, opt := range opts {
		opt(hc.options)
	}
	hc.c = &http.Client{Timeout: hc.options.timeout}
	return hc
}

func (hc *HttpClient) getRequestURL(reqUrl string, querys map[string]string) string {
	vals := url.Values{}
	for k, v := range querys {
		vals[k] = []string{v}
	}
	if len(hc.options.baseURL) > 0 {
		reqUrl = strings.TrimLeft(reqUrl, "/")
		if strings.HasSuffix(hc.options.baseURL, "/") {
			reqUrl = hc.options.baseURL + reqUrl
		} else if reqUrl != "" {
			reqUrl = hc.options.baseURL + "/" + reqUrl
		} else {
			reqUrl = hc.options.baseURL
		}
	}
	if len(vals) > 0 {
		if strings.Index(reqUrl, "?") > 0 {
			return reqUrl + "&" + vals.Encode()
		} else {
			return reqUrl + "?" + vals.Encode()
		}
	}
	return reqUrl
}

func (hc *HttpClient) Request(ctx context.Context, method, url string, querys map[string]string, reqObj, rspObj interface{}, handlers ...RequestHandler) error {
	var reader io.Reader = nil
	if raw, ok := reqObj.(string); ok {
		reader = bytes.NewBuffer([]byte(raw))
	} else if reqReader, ok := reqObj.(io.Reader); ok && reqReader != nil {
		reader = reqReader
	} else if reqObj != nil {
		bodyBytes, err := hc.options.encoder.Encode(reqObj)
		if err != nil {
			return err
		}
		reader = bytes.NewBuffer(bodyBytes)
	}
	reqUrl := hc.getRequestURL(url, querys)
	headers := GetHeaders(ctx)
	req, err := http.NewRequest(method, reqUrl, reader)
	if err != nil {
		return err
	}
	if ctx != nil {
		req = req.WithContext(ctx)
	}
	if hc.options.encoder == jsonEncoder || hc.options.decoder == jsonDecoder {
		req.Header.Add("Content-Type", "application/json")
	}
	if len(headers) > 0 {
		for k, v := range headers {
			req.Header.Add(k, v)
		}
	}
	for _, h := range hc.options.handlers {
		if err = h(req); err != nil {
			return err
		}
	}
	for _, h := range handlers {
		if err = h(req); err != nil {
			return err
		}
	}
	rsp, err := hc.c.Do(req)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()
	rspBody, err := io.ReadAll(rsp.Body)
	if err != nil {
		return err
	}
	if lo.IndexOf(hc.options.successCodes, rsp.StatusCode) < 0 {
		if len(rspBody) > 128 {
			rspBody = append(rspBody[:128], '.', '.', '.')
		}
		return &Error{StatusCode: rsp.StatusCode, Body: string(rspBody)}
	}
	err = hc.options.decoder.Decode(rspBody, rspObj)
	if err != nil {
		return err
	}
	return err
}

func (hc *HttpClient) POST(ctx context.Context, url string, reqObj, rspObj interface{}, handlers ...RequestHandler) error {
	return hc.Request(ctx, "POST", url, nil, reqObj, rspObj, handlers...)
}

func (hc *HttpClient) GET(ctx context.Context, url string, querys map[string]string, rspObj interface{}, handlers ...RequestHandler) error {
	return hc.Request(ctx, "GET", url, querys, nil, rspObj, handlers...)
}
