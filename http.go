package fasthttp

import (
	"errors"
	"fmt"
	"github.com/dop251/goja"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib/netext/httpext"
	"sync"
)

type RootModule struct{}

// ModuleInstance represents an instance of the HTTP module for every VU.
type ModuleInstance struct {
	vu      modules.VU
	exports *goja.Object
}

var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}
)

func init() {
	modules.Register("blahk6/x/fasthttp", New())
}

// New returns a pointer to a new HTTP RootModule.
func New() *RootModule {
	return &RootModule{}
}

// NewModuleInstance returns an HTTP module instance for each VU.
func (r *RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	rt := vu.Runtime()

	mi := &ModuleInstance{
		vu:      vu,
		exports: rt.NewObject(),
	}

	mustExport := func(name string, value interface{}) {
		if err := mi.exports.Set(name, value); err != nil {
			common.Throw(rt, err)
		}
	}

	mustExport("FileStream", mi.FileStream)
	mustExport("Client", mi.Client)
	mustExport("Request", mi.Request)
	mustExport("checkstatus", mi.CheckStatus)

	return mi
}

// RequestWrapper Create new request with New RequestWrapper({})
func (mi *ModuleInstance) Request(call goja.ConstructorCall, rt *goja.Runtime) *goja.Object {
	if len(call.Arguments) > 2 || len(call.Arguments) == 0 {
		common.Throw(mi.vu.Runtime(), errors.New("req constructor expects 1 or 2 args"))
	}

	var req RequestWrapper
	req.Url = call.Arguments[0].String()
	req.reqPool = &sync.Pool{}

	if len(call.Arguments) > 1 {
		err := rt.ExportTo(call.Argument(1), &req)
		if err != nil {
			common.Throw(rt, fmt.Errorf("request constructor expects first argument to be RequestWrapper got error %v", err))
		}

		if req.ResponseType != "" {
			responseType, err := httpext.ResponseTypeString(req.ResponseType)
			if err != nil {
				common.Throw(mi.vu.Runtime(), fmt.Errorf("Invalid response type %v", err))
			}
			req.responseType = responseType
		}
	}

	return mi.vu.Runtime().ToValue(&req).ToObject(rt)
}

// Exports returns the JS values this module exports.
func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Default: mi.exports,
	}
}
