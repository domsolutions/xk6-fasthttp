package metrics

import (
	"context"
	"net"
	"strconv"
	"sync"

	"github.com/domsolutions/xk6-fasthttp/errors"
	"github.com/domsolutions/xk6-fasthttp/tracer"
	http "github.com/valyala/fasthttp"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/lib/netext"
	"go.k6.io/k6/metrics"
)

// UnfinishedRequest stores the Request and the raw result returned from the
// underlying http.RoundTripper, but before its body has been read
type UnfinishedRequest struct {
	Ctx      context.Context
	Trail    *tracer.Trail
	Request  *http.Request
	Response *http.Response
	Err      error
}

type FinishedRequest struct {
	*UnfinishedRequest
	Trail     *tracer.Trail
	TLSInfo   netext.TLSInfo
	ErrorCode errors.ErrCode
	ErrorMsg  string
}

type MetricDispatcher struct {
	State            *lib.State
	TagsAndMeta      *metrics.TagsAndMeta
	responseCallback func(int) bool

	lastRequest     *UnfinishedRequest
	lastRequestLock *sync.Mutex
}

func NewMetricDispatcher(tags *metrics.TagsAndMeta, state *lib.State) *MetricDispatcher {
	return &MetricDispatcher{TagsAndMeta: tags, State: state, lastRequestLock: &sync.Mutex{}}
}

func (t *MetricDispatcher) ProcessLastSavedRequest(ctx context.Context, lastErr error) *FinishedRequest {
	t.lastRequestLock.Lock()
	unprocessedRequest := t.lastRequest
	t.lastRequest = nil
	t.lastRequestLock.Unlock()

	if unprocessedRequest != nil {
		// We don't want to overwrite any previous errors, but if there were
		// none and we (i.e. the MakeRequest() function) have one, save it
		// before we emit the metrics.
		if unprocessedRequest.Err == nil && lastErr != nil {
			unprocessedRequest.Err = lastErr
		}

		return t.measureAndEmitMetrics(ctx, unprocessedRequest)
	}
	return nil
}

func (t *MetricDispatcher) SaveCurrentRequest(ctx context.Context, currentRequest *UnfinishedRequest) {
	t.lastRequestLock.Lock()
	unprocessedRequest := t.lastRequest
	t.lastRequest = currentRequest
	t.lastRequestLock.Unlock()

	if unprocessedRequest != nil {
		// This shouldn't happen, since we have one transport per Request, but just in case...
		t.State.Logger.Warnf("TracerTransport: unexpected unprocessed Request for %s", unprocessedRequest.Request.URI().String())
		t.measureAndEmitMetrics(ctx, unprocessedRequest)
	}
}

// Helper method to finish the Tracer Trail, assemble the tag values and emits
// the metric samples for the supplied unfinished Request.
//
//nolint:funlen
func (t *MetricDispatcher) measureAndEmitMetrics(ctx context.Context, unfReq *UnfinishedRequest) *FinishedRequest {
	trail := unfReq.Trail

	result := &FinishedRequest{
		UnfinishedRequest: unfReq,
		Trail:             trail,
	}

	tagsAndMeta := t.TagsAndMeta.Clone()
	enabledTags := t.State.Options.SystemTags

	// After k6 v0.41.0, the `name` and `url` tags have the exact same values:
	nameTagValue, nameTagManuallySet := tagsAndMeta.Tags.Get(metrics.TagName.String())
	if !nameTagManuallySet {
		// If the user *didn't* manually set a `name` tag value and didn't use
		// the http.url template literal helper to have k6 automatically set
		// it (see `lib/netext/httpext.MakeRequest()`), we will use the cleaned
		// URL value as the value of both `name` and `url` tags.
		uri := unfReq.Request.URI().String()
		tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabledTags, metrics.TagName, uri)
		tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabledTags, metrics.TagURL, uri)
	} else {
		// However, if the user set the `name` tag value somehow, we will use
		// whatever they set as the value of the `url` tags too, to prevent
		// high-cardinality values in the indexed tags.
		tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabledTags, metrics.TagURL, nameTagValue)
	}

	tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabledTags, metrics.TagMethod, string(unfReq.Request.Header.Method()))

	if unfReq.Err != nil {
		result.ErrorCode, result.ErrorMsg = errors.ErrorCodeForError(unfReq.Err)
		tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabledTags, metrics.TagError, result.ErrorMsg)
		tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabledTags, metrics.TagErrorCode, strconv.Itoa(int(result.ErrorCode)))
		tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabledTags, metrics.TagStatus, "0")
	} else {
		tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabledTags, metrics.TagStatus, strconv.Itoa(unfReq.Response.StatusCode()))
		if unfReq.Response.StatusCode() >= 400 {
			result.ErrorCode = errors.ErrCode(1000 + unfReq.Response.StatusCode())
			tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabledTags, metrics.TagErrorCode, strconv.Itoa(int(result.ErrorCode)))
		}
	}

	if enabledTags.Has(metrics.TagIP) && trail.ConnRemoteAddr != nil {
		if ip, _, err := net.SplitHostPort(trail.ConnRemoteAddr.String()); err == nil {
			tagsAndMeta.SetSystemTagOrMeta(metrics.TagIP, ip)
		}
	}
	var failed float64
	if t.responseCallback != nil {
		var statusCode int
		if unfReq.Err == nil {
			statusCode = unfReq.Response.StatusCode()
		}
		expected := t.responseCallback(statusCode)
		if !expected {
			failed = 1
		}

		tagsAndMeta.SetSystemTagOrMetaIfEnabled(enabledTags, metrics.TagExpectedResponse, strconv.FormatBool(expected))
	}

	trail.SaveSamples(t.State.BuiltinMetrics, &tagsAndMeta)
	if t.responseCallback != nil {
		trail.Failed.Valid = true
		if failed == 1 {
			trail.Failed.Bool = true
		}
		trail.Samples = append(trail.Samples,
			metrics.Sample{
				TimeSeries: metrics.TimeSeries{
					Metric: t.State.BuiltinMetrics.HTTPReqFailed,
					Tags:   tagsAndMeta.Tags,
				},
				Time:     trail.EndTime,
				Metadata: tagsAndMeta.Metadata,
				Value:    failed,
			},
		)
	}

	metrics.PushIfNotDone(ctx, t.State.Samples, trail)
	return result
}
