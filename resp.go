package fasthttp

import (
	"fmt"

	http "github.com/valyala/fasthttp"
	"go.k6.io/k6/lib/netext/httpext"
)

func readResponseBody(respType httpext.ResponseType, resp *http.Response) (interface{}, error) {
	// Ensure that the entire response body is read and closed so conn can be reused
	defer func() {
		_ = resp.Body()
		resp.CloseBodyStream()
	}()

	if respType == httpext.ResponseTypeNone {
		return nil, nil
	}

	if (resp.StatusCode() >= 100 && resp.StatusCode() <= 199) || // 1xx
		resp.StatusCode() == http.StatusNoContent || resp.StatusCode() == http.StatusNotModified {
		// for all three of this status code there is always no content
		// https://www.rfc-editor.org/rfc/rfc9110.html#section-6.4.1-8
		// this also prevents trying to read
		return nil, nil //nolint:nilnil
	}

	var result interface{}
	// Binary or string
	switch respType {
	case httpext.ResponseTypeText:
		result = string(resp.Body())
	case httpext.ResponseTypeBinary:
		result = resp.Body()
	default:
		return nil, fmt.Errorf("unknown responseType %s", respType)
	}

	return result, nil
}
