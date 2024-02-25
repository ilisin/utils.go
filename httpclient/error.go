package httpclient

import (
	"fmt"
)

type Error struct {
	StatusCode int    // http status code, not contain success code
	Body       string // bytes size limit to 128
}

func (e *Error) Error() string {
	return fmt.Sprintf("status code: %d, body: %s", e.StatusCode, e.Body)
}
