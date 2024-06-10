package fasthttp

import (
	"errors"
	"os"

	"github.com/grafana/sobek"
	"go.k6.io/k6/js/common"
)

type FileStream struct {
	*os.File
}

func (s *FileStream) Close() error {
	// prevent file from being closed so it can stream for multiple requests as fasthttp calls Close()
	// after sending body
	return nil
}

func (mi *ModuleInstance) FileStream(call sobek.ConstructorCall, rt *sobek.Runtime) *sobek.Object {
	if len(call.Arguments) != 1 {
		common.Throw(rt, errors.New("one arg required of file path for stream"))
	}

	f, err := os.Open(call.Argument(0).String())
	if err != nil {
		mi.vu.State().Logger.WithError(err).Errorf("Failed to open file %s", call.Argument(0).String())
		common.Throw(rt, err)
	}

	return rt.ToValue(&FileStream{f}).ToObject(rt)
}
