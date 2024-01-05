package errors

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"runtime"
	"syscall"

	"golang.org/x/net/http2"

	"go.k6.io/k6/lib/netext"
)

// TODO: maybe rename the type errorCode, so we can have ErrCode variables? and
// also the constants would probably be better of if `ErrorCode` was a prefix,
// not a suffix - they would be much easier for auto-autocompletion at least...

type ErrCode uint32

const (
	// non specific
	defaultErrorCode          ErrCode = 1000
	defaultNetNonTCPErrorCode ErrCode = 1010
	invalidURLErrorCode       ErrCode = 1020
	requestTimeoutErrorCode   ErrCode = 1050
	// DNS errors
	defaultDNSErrorCode      ErrCode = 1100
	dnsNoSuchHostErrorCode   ErrCode = 1101
	blackListedIPErrorCode   ErrCode = 1110
	blockedHostnameErrorCode ErrCode = 1111
	// tcp errors
	defaultTCPErrorCode      ErrCode = 1200
	tcpBrokenPipeErrorCode   ErrCode = 1201
	netUnknownErrnoErrorCode ErrCode = 1202
	tcpDialErrorCode         ErrCode = 1210
	tcpDialTimeoutErrorCode  ErrCode = 1211
	tcpDialRefusedErrorCode  ErrCode = 1212
	tcpDialUnknownErrnoCode  ErrCode = 1213
	tcpResetByPeerErrorCode  ErrCode = 1220
	// TLS errors
	defaultTLSErrorCode           ErrCode = 1300 //nolint:deadcode,varcheck // this is here to save the number
	tlsHeaderErrorCode            ErrCode = 1301
	x509UnknownAuthorityErrorCode ErrCode = 1310
	x509HostnameErrorCode         ErrCode = 1311

	// HTTP2 errors
	// defaultHTTP2ErrorCode ErrCode = 1600 // commented because of golint
	// HTTP2 GoAway errors
	unknownHTTP2GoAwayErrorCode ErrCode = 1610
	// errors till 1611 + 13 are other HTTP2 GoAway errors with a specific ErrCode

	// HTTP2 FileStream errors
	unknownHTTP2StreamErrorCode ErrCode = 1630
	// errors till 1631 + 13 are other HTTP2 FileStream errors with a specific ErrCode

	// HTTP2 Connection errors
	unknownHTTP2ConnectionErrorCode ErrCode = 1650
	// errors till 1651 + 13 are other HTTP2 Connection errors with a specific ErrCode

	// Custom k6 content errors, i.e. when the magic fails
	// defaultContentError ErrCode = 1700 // reserved for future use
	responseDecompressionErrorCode ErrCode = 1701
)

const (
	tcpResetByPeerErrorCodeMsg  = "%s: connection reset by peer"
	tcpDialTimeoutErrorCodeMsg  = "dial: i/o timeout"
	tcpDialRefusedErrorCodeMsg  = "dial: connection refused"
	tcpBrokenPipeErrorCodeMsg   = "%s: broken pipe"
	netUnknownErrnoErrorCodeMsg = "%s: unknown errno `%d` on %s with message `%s`"
	dnsNoSuchHostErrorCodeMsg   = "lookup: no such host"
	blackListedIPErrorCodeMsg   = "ip is blacklisted"
	blockedHostnameErrorMsg     = "hostname is blocked"
	http2GoAwayErrorCodeMsg     = "http2: received GoAway with http2 ErrCode %s"
	http2StreamErrorCodeMsg     = "http2: stream error with http2 ErrCode %s"
	http2ConnectionErrorCodeMsg = "http2: connection error with http2 ErrCode %s"
	x509HostnameErrorCodeMsg    = "x509: certificate doesn't match hostname"
	x509UnknownAuthority        = "x509: unknown authority"
	requestTimeoutErrorCodeMsg  = "request timeout"
	invalidURLErrorCodeMsg      = "invalid URL"
)

func http2ErrCodeOffset(code http2.ErrCode) ErrCode {
	if code > http2.ErrCodeHTTP11Required {
		return 0
	}
	return 1 + ErrCode(code)
}

//nolint:errorlint
func errorCodeForNetOpError(err *net.OpError) (ErrCode, string) {
	// TODO: refactor this further - a big switch would be more readable, maybe
	// we should even check for *os.SyscallError in the main switch body in the
	// parent errorCodeForError() function?

	if err.Net != "tcp" && err.Net != "tcp6" {
		// TODO: figure out how this happens
		return defaultNetNonTCPErrorCode, err.Error()
	}
	if sErr, ok := err.Err.(*os.SyscallError); ok {
		switch sErr.Unwrap() {
		case syscall.ECONNRESET:
			return tcpResetByPeerErrorCode, fmt.Sprintf(tcpResetByPeerErrorCodeMsg, err.Op)
		case syscall.EPIPE:
			return tcpBrokenPipeErrorCode, fmt.Sprintf(tcpBrokenPipeErrorCodeMsg, err.Op)
		}
		code, msg := getOSSyscallErrorCode(err, sErr)
		if code != 0 {
			return code, msg
		}
	}
	if err.Op != "dial" {
		switch inErr := err.Err.(type) {
		case syscall.Errno:
			return netUnknownErrnoErrorCode,
				fmt.Sprintf(netUnknownErrnoErrorCodeMsg,
					err.Op, (int)(inErr), runtime.GOOS, inErr.Error())
		default:
			return defaultTCPErrorCode, err.Error()
		}
	}

	if iErr, ok := err.Err.(*os.SyscallError); ok {
		if errno, ok := iErr.Err.(syscall.Errno); ok {
			if errno == syscall.ECONNREFUSED ||
				// 10061 is some connection refused like thing on windows
				// TODO: fix by moving to x/sys instead of syscall after
				// https://github.com/golang/go/issues/31360 gets resolved
				(errno == 10061 && runtime.GOOS == "windows") {
				return tcpDialRefusedErrorCode, tcpDialRefusedErrorCodeMsg
			}
			return tcpDialUnknownErrnoCode,
				fmt.Sprintf("dial: unknown errno %d error with msg `%s`", errno, iErr.Err)
		}
	}

	// Check if the wrapped error isn't something we recognize, e.g. a DNS error
	if wrappedErr := errors.Unwrap(err); wrappedErr != nil {
		errCodeForWrapped, errForWrapped := ErrorCodeForError(wrappedErr)
		if errCodeForWrapped != defaultErrorCode {
			return errCodeForWrapped, errForWrapped
		}
	}

	// If it's not, return a generic TCP dial error
	return tcpDialErrorCode, err.Error()
}

// errorCodeForError returns the errorCode and a specific error message for given error.
//
//nolint:errorlint
func ErrorCodeForError(err error) (ErrCode, string) {
	// We explicitly check for `Unwrap()` in the default switch branch, but
	// checking for the concrete error types first gives us the opportunity to
	// also directly detect high-level errors, if we need to, even if they wrap
	// a low level error inside.
	switch e := err.(type) {
	case K6Error:
		return e.Code, e.Message
	case *net.DNSError:
		switch e.Err {
		case "no such host": // defined as private in the go stdlib
			return dnsNoSuchHostErrorCode, dnsNoSuchHostErrorCodeMsg
		default:
			return defaultDNSErrorCode, err.Error()
		}
	case netext.BlackListedIPError:
		return blackListedIPErrorCode, blackListedIPErrorCodeMsg
	case netext.BlockedHostError:
		return blockedHostnameErrorCode, blockedHostnameErrorMsg
	case http2.GoAwayError:
		return unknownHTTP2GoAwayErrorCode + http2ErrCodeOffset(e.ErrCode),
			fmt.Sprintf(http2GoAwayErrorCodeMsg, e.ErrCode)
	case http2.StreamError:
		return unknownHTTP2StreamErrorCode + http2ErrCodeOffset(e.Code),
			fmt.Sprintf(http2StreamErrorCodeMsg, e.Code)
	case http2.ConnectionError:
		return unknownHTTP2ConnectionErrorCode + http2ErrCodeOffset(http2.ErrCode(e)),
			fmt.Sprintf(http2ConnectionErrorCodeMsg, http2.ErrCode(e))
	case *net.OpError:
		return errorCodeForNetOpError(e)
	case x509.UnknownAuthorityError:
		return x509UnknownAuthorityErrorCode, x509UnknownAuthority
	case x509.HostnameError:
		return x509HostnameErrorCode, x509HostnameErrorCodeMsg
	case tls.RecordHeaderError:
		return tlsHeaderErrorCode, err.Error()
	case *url.Error:
		return ErrorCodeForError(e.Err)
	default:
		if wrappedErr := errors.Unwrap(err); wrappedErr != nil {
			return ErrorCodeForError(wrappedErr)
		}

		return defaultErrorCode, err.Error()
	}
}

// K6Error is a helper struct that enhances Go errors with custom k6-specific
// error-codes and more user-readable error messages.
type K6Error struct {
	Code          ErrCode
	Message       string
	OriginalError error
}

// NewK6Error is the constructor for K6Error
func NewK6Error(code ErrCode, msg string, originalErr error) K6Error {
	return K6Error{code, msg, originalErr}
}

// Error implements the `error` interface, so K6Errors are normal Go errors.
func (k6Err K6Error) Error() string {
	return k6Err.Message
}

// Unwrap implements the `xerrors.Wrapper` interface, so K6Errors are a bit
// future-proof Go 2 errors.
func (k6Err K6Error) Unwrap() error {
	return k6Err.OriginalError
}
