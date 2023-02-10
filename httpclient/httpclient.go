package httpclient

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const DefaultTimeout = 5 * time.Second

type HttpClient struct {
	baseURL         string
	requestEncoder  Encoder
	responseDecoder Decoder
	log             bool

	c *http.Client
}

func New(options ...Option) *HttpClient {
	hc := new(HttpClient)
	hc.responseDecoder = defaultDecoder
	hc.requestEncoder = defaultEncoder
	hc.log = true
	hc.c = &http.Client{Timeout: DefaultTimeout}
	for _, o := range options {
		o.SetOption(hc)
	}
	return hc
}

func (hc *HttpClient) SetOption(opt Option) {
	opt.SetOption(hc)
}

func (hc *HttpClient) getRequestURL(reqUrl string, querys map[string]string) string {
	vals := url.Values{}
	for k, v := range querys {
		vals[k] = []string{v}
	}
	if len(hc.baseURL) > 0 {
		reqUrl = strings.TrimLeft(reqUrl, "/")
		if strings.HasSuffix(hc.baseURL, "/") {
			reqUrl = hc.baseURL + reqUrl
		} else {
			reqUrl = hc.baseURL + "/" + reqUrl
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

func (hc *HttpClient) Request(method, url string, querys map[string]string, reqObj, rspObj interface{}, handlers ...RequestHandler) error {
	var reader io.Reader = nil
	if reqObj != nil {
		bodyBytes, err := hc.requestEncoder.Encode(reqObj)
		if err != nil {
			return err
		}
		reader = bytes.NewBuffer(bodyBytes)
	}
	reqUrl := hc.getRequestURL(url, querys)
	req, err := http.NewRequest(method, hc.getRequestURL(url, querys), reader)
	if err != nil {
		return err
	}

	log := logrus.Fields{
		"url":         reqUrl,
		"requestBody": reqObj,
	}
	// setup request
	if hc.requestEncoder == jsonEncoder || hc.responseDecoder == jsonDecoder {
		req.Header.Add("Content-Type", "application/json")
	}
	for _, h := range handlers {
		h(req)
	}
	startAt := time.Now()
	rsp, err := hc.c.Do(req)
	if err != nil {
		log["responseErr"] = err
		logrus.WithFields(log).Error("HTTPCLIENT request error")
		return err
	}
	defer rsp.Body.Close()
	log["responseStatus"] = rsp.StatusCode
	log["duration"] = time.Now().Sub(startAt)
	rspBody, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		log["bodyReadErr"] = err
		logrus.WithFields(log).Error("HTTPCLIENT request error")
		return err
	}
	err = hc.responseDecoder.Decode(rspBody, rspObj)
	if err != nil {
		log["decodeErr"] = err
		log["responseBody"] = string(rspBody)
		logrus.WithFields(log).Error("HTTPCLIENT request error")
		return err
	}
	if hc.log {
		log["responseBody"] = string(rspBody)
		logrus.WithFields(log).Debug("HTTPCLIENT request ok")
	}
	return err
}

func (hc *HttpClient) POST(url string, reqObj, rspObj interface{}, handlers ...RequestHandler) error {
	return hc.Request("POST", url, nil, reqObj, rspObj, handlers...)
}

func (hc *HttpClient) GET(url string, querys map[string]string, rspObj interface{}, handlers ...RequestHandler) error {
	return hc.Request("GET", url, querys, nil, rspObj, handlers...)
}
