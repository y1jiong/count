package cfg

import (
	"context"
	"github.com/gogf/gf/v2/container/gvar"
	"github.com/gogf/gf/v2/os/gcfg"
)

func get(ctx context.Context, path string, def any) *gvar.Var {
	val, err := gcfg.Instance().Get(ctx, path, def)
	if err != nil || val == nil {
		return gvar.New(def)
	}
	return val
}
