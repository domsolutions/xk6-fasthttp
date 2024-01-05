//go:build !windows
// +build !windows

package errors

import (
	"net"
	"os"
)

func getOSSyscallErrorCode(e *net.OpError, se *os.SyscallError) (ErrCode, string) {
	return 0, ""
}
