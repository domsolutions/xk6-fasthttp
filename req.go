package fasthttp

import (
	"github.com/valyala/fasthttp"
	"go.k6.io/k6/lib/netext/httpext"
	"sync"
)

type RequestWrapper struct {
	Throw            bool
	DisableKeepAlive bool
	Url              string
	Host             string
	Headers          map[string]string
	Body             interface{}
	req              *fasthttp.Request
	reqPool          *sync.Pool
	ResponseType     string
	responseType     httpext.ResponseType
}
