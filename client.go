package fasthttp

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	e "github.com/domsolutions/xk6-fasthttp/errors"
	"github.com/domsolutions/xk6-fasthttp/metrics"
	"github.com/domsolutions/xk6-fasthttp/tracer"
	"github.com/dop251/goja"
	http "github.com/valyala/fasthttp"
	proxy "github.com/valyala/fasthttp/fasthttpproxy"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib/netext/httpext"
	"net"
	"sync"
	"time"
)

const (
	defaultDialTimeout     = 5 * time.Second
	defaultMaxConnsPerHost = 1
)

type ClientConfig struct {
	DialTimeout     int
	Proxy           string
	MaxConnDuration int
	UserAgent       string
	ReadBufferSize  int
	WriteBufferSize int
	ReadTimeout     int
	WriteTimeout    int
	MaxConnsPerHost int
	TLSConfig       TLSConfig
}

type TLSConfig struct {
	InsecureSkipVerify bool
	PrivateKey         string
	Certificate        string
}

type Client struct {
	fhc              *http.Client
	vu               modules.VU
	metrics          *metrics.MetricDispatcher
	metricsSetupOnce *sync.Once
}

func (mi *ModuleInstance) Client(call goja.ConstructorCall, rt *goja.Runtime) *goja.Object {
	if mi.vu.State() != nil {
		common.Throw(rt, errors.New("creating client objects is allowed only in the Init context"))
	}

	var config ClientConfig
	err := rt.ExportTo(call.Argument(0), &config)
	if err != nil {
		common.Throw(rt, fmt.Errorf("client constructor expects first argument to be ClientConfig got error %v", err))
	}

	var fhc *http.Client
	if fhc, err = parseClientConfig(config); err != nil {
		common.Throw(rt, err)
	}

	c := &Client{fhc: fhc, vu: mi.vu, metricsSetupOnce: &sync.Once{}}
	return rt.ToValue(c).ToObject(rt)
}

func parseClientConfig(config ClientConfig) (*http.Client, error) {
	if config.TLSConfig.PrivateKey != "" && config.TLSConfig.Certificate == "" {
		return nil, errors.New("blank certificate")
	}
	if config.TLSConfig.PrivateKey == "" && config.TLSConfig.Certificate != "" {
		return nil, errors.New("blank private key")
	}
	tlsConfig := &tls.Config{
		InsecureSkipVerify: config.TLSConfig.InsecureSkipVerify,
	}

	if config.TLSConfig.Certificate != "" && config.TLSConfig.PrivateKey != "" {
		cert, err := tls.LoadX509KeyPair(config.TLSConfig.Certificate, config.TLSConfig.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to load key/cert; %v", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	maxConnsPerHost := defaultMaxConnsPerHost
	if config.MaxConnsPerHost > 0 {
		maxConnsPerHost = config.MaxConnsPerHost
	}

	fhc := &http.Client{
		Name:                          config.UserAgent,
		MaxConnDuration:               time.Duration(config.MaxConnDuration) * time.Second,
		ReadBufferSize:                config.ReadBufferSize,
		WriteBufferSize:               config.WriteBufferSize,
		WriteTimeout:                  time.Duration(config.WriteTimeout) * time.Second,
		ReadTimeout:                   time.Duration(config.ReadTimeout) * time.Second,
		MaxConnsPerHost:               maxConnsPerHost,
		DisableHeaderNamesNormalizing: true,
		TLSConfig:                     tlsConfig,
		Dial: func(addr string) (net.Conn, error) {
			timeout := defaultDialTimeout
			if config.DialTimeout > 0 {
				timeout = time.Duration(config.DialTimeout) * time.Second
			}
			if config.Proxy != "" {
				return proxy.FasthttpHTTPDialerTimeout(config.Proxy, timeout)(addr)
			}
			return http.DialTimeout(addr, timeout)
		},
	}

	return fhc, nil
}

func (c *Client) verifyReq(r *goja.Object) {
	if _, ok := r.Export().(*RequestWrapper); !ok {
		common.Throw(c.vu.Runtime(), errors.New("object not a Request"))
	}
}

func (c *Client) Options(r *goja.Object) (*Response, error) {
	c.verifyReq(r)
	return c.makeReq(r.Export().(*RequestWrapper), http.MethodOptions)
}

func (c *Client) Put(r *goja.Object) (*Response, error) {
	c.verifyReq(r)
	return c.makeReq(r.Export().(*RequestWrapper), http.MethodPut)
}

func (c *Client) Patch(r *goja.Object) (*Response, error) {
	c.verifyReq(r)
	return c.makeReq(r.Export().(*RequestWrapper), http.MethodPatch)
}

func (c *Client) Delete(r *goja.Object) (*Response, error) {
	c.verifyReq(r)
	return c.makeReq(r.Export().(*RequestWrapper), http.MethodDelete)
}

func (c *Client) Post(r *goja.Object) (*Response, error) {
	c.verifyReq(r)
	return c.makeReq(r.Export().(*RequestWrapper), http.MethodPost)
}

func (c *Client) Get(r *goja.Object) (*Response, error) {
	c.verifyReq(r)
	return c.makeReq(r.Export().(*RequestWrapper), http.MethodGet)
}

func setBody(method string, body interface{}) bool {
	return body != nil && method != http.MethodHead && method != http.MethodGet
}

func (c *Client) setupCachedReq(reqw *RequestWrapper, method string) error {
	if setBody(method, reqw.Body) {

		switch reqw.Body.(type) {
		case *FileStream:
			f := reqw.Body.(*FileStream)
			// reset to beginning of file
			if _, err := f.Seek(0, 0); err != nil {
				c.vu.State().Logger.WithError(err).Error("Failed to reset stream to beginning")
				return err
			}
			reqw.req.SetBodyStream(f, -1)
		}

		return nil
	}

	// reset body as req may be a GET request which should have no body but cached req may have a body
	reqw.req.SetBody(nil)
	reqw.req.SetBodyStream(nil, 0)
	reqw.req.Header.SetMethod(method)
	return nil
}

func (c *Client) setupNewReq(reqw *RequestWrapper, method string) error {
	reqw.req.SetRequestURI(reqw.Url)

	if reqw.Host != "" {
		reqw.req.UseHostHeader = true
		reqw.req.Header.SetHost(reqw.Host)
	}

	if setBody(method, reqw.Body) {

		switch reqw.Body.(type) {
		case string:
			reqw.req.SetBody([]byte(reqw.Body.(string)))
		case goja.ArrayBuffer:
			reqw.req.SetBody(reqw.Body.(goja.ArrayBuffer).Bytes())
		case *FileStream:
			f := reqw.Body.(*FileStream)
			// reset to beginning of file for fresh request
			if _, err := f.Seek(0, 0); err != nil {
				c.vu.State().Logger.WithError(err).Error("Failed to reset stream to beginning")
				return err
			}
			reqw.req.SetBodyStream(f, -1)
		default:
			return errors.New("req body type not supported")
		}
	}

	if reqw.DisableKeepAlive {
		reqw.req.Header.SetConnectionClose()
	}
	for field, val := range reqw.Headers {
		reqw.req.Header.Set(field, val)
	}

	reqw.req.Header.SetMethod(method)
	return nil
}

func (c *Client) makeReq(req *RequestWrapper, method string) (*Response, error) {
	if r := req.reqPool.Get(); r != nil {
		req.req = r.(*http.Request)
		if err := c.setupCachedReq(req, method); err != nil {
			return nil, err
		}
	} else {
		req.req = http.AcquireRequest()
		if err := c.setupNewReq(req, method); err != nil {
			return nil, err
		}
	}

	defer func() {
		req.reqPool.Put(req.req)
		req.req = nil
	}()

	c.metricsSetupOnce.Do(func() {
		tags := c.vu.State().Tags.GetCurrentValues()
		c.metrics = metrics.NewMetricDispatcher(&tags, c.vu.State())
	})

	var resp *Response
	var err error
	if resp, err = c.do(c.vu.Context(), req); err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *Client) do(ctx context.Context, req *RequestWrapper) (response *Response, err error) {
	resp := http.AcquireResponse()

	defer func() {
		http.ReleaseResponse(resp)
		if !req.Throw {
			err = nil
		}
	}()

	c.metrics.ProcessLastSavedRequest(c.vu.Context(), nil)

	t1 := time.Now()
	// send request on wire
	err = c.fhc.Do(req.req, resp)
	trial := &tracer.Trail{Duration: time.Since(t1)}

	c.metrics.SaveCurrentRequest(c.vu.Context(), &metrics.UnfinishedRequest{
		Ctx:      ctx,
		Trail:    trial,
		Request:  req.req,
		Response: resp,
		Err:      err,
	})

	if err != nil {
		if !req.Throw {
			c.vu.State().Logger.WithError(err).Warn("Request Failed")
		}
		return nil, err
	}

	r := &httpext.Response{}
	r.Status = resp.StatusCode()
	r.RemoteIP = resp.RemoteAddr().String()
	r.URL = req.req.URI().String()

	r.Headers = make(map[string]string)
	resp.Header.VisitAll(func(key, value []byte) {
		r.Headers[string(key)] = string(value)
	})

	response = &Response{Response: r, client: c}

	response.Body, err = readResponseBody(req.responseType, resp)
	if err != nil {
		var code e.ErrCode
		code, response.Error = e.ErrorCodeForError(err)
		response.ErrorCode = int(code)
		return response, err
	}

	return response, nil
}
