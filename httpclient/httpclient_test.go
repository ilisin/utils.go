package httpclient

import (
	"net/http"
	"testing"
	"time"
)

type Req struct {
	MsgType string `json:"msg_type"`
	Content struct {
		Text string `json:"text"`
	} `json:"content"`
}
type Rsp struct {
	StatusCode    int
	StatusMessage string
}

func TestPost(t *testing.T) {

	c := New(NewBaseURLOption("https://www.cip.cc"), NewTimeoutOption(10*time.Second), NewRequestHandlerOption(func(r *http.Request) error {
		r.Header.Set("User-Agent", "curl/7.68.0")
		return nil
	}))

	req := &Req{
		MsgType: "text",
	}
	req.Content.Text = "hello, im from go-code http client"
	var rsp = &Rsp{}
	if err := c.POST("/bot/v2/hook/xxx", req, rsp); err != nil {
		t.Fatal(err)
	}
}
