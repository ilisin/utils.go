package httpclient

import (
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

	c := New(NewBaseURLOption("https://open.feishu.cn/open-apis"), NewTimeoutOption(10*time.Second))

	req := &Req{
		MsgType: "text",
		// Content: struct{ Text string }{Text: "hello"},
	}
	req.Content.Text = "hello, im from go-code http client"
	var rsp = &Rsp{}
	if err := c.POST("/bot/v2/hook/e6708fcd-fb51-4782-a387-12b39366eb31", req, rsp); err != nil {
		t.Fatal(err)
	}
}
