package service

import (
	"context"
	"github.com/gogf/gf/v2/os/gcfg"
)

func MapAbbr(ctx context.Context, abbr string) string {
	v, err := gcfg.Instance().Get(ctx, "abbr."+abbr)
	if err != nil || v == nil {
		return abbr
	}
	return v.String()
}
