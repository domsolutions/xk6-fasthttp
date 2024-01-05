package errors

import (
	"fmt"
	"net"
	"os"
	"syscall"
)

func getOSSyscallErrorCode(e *net.OpError, se *os.SyscallError) (ErrCode, string) {
	switch se.Unwrap() {
	case syscall.WSAECONNRESET:
		return tcpResetByPeerErrorCode, fmt.Sprintf(tcpResetByPeerErrorCodeMsg, e.Op)
	}
	return 0, ""
}
