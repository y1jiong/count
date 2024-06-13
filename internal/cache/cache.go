package cache

import (
	"context"
	"github.com/gogf/gf/v2/container/gvar"
	"github.com/gogf/gf/v2/os/gcache"
	"github.com/gogf/gf/v2/os/gtime"
	"time"
)

var cache = gcache.New()

func Get(ctx context.Context, key any) (*gvar.Var, error) {
	val, err := cache.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	if val == nil {
		m := loadFile(ctx)
		now := gtime.Now()
		err = cache.SetMap(ctx, m, now.AddDate(0, 0, 1).EndOfDay().Sub(now))
		if err != nil {
			return nil, err
		}
		val, err = cache.Get(ctx, key)
	}
	return val, err
}

func Set(ctx context.Context, key any, value any, duration time.Duration) error {
	err := cache.Set(ctx, key, value, duration)
	if err != nil {
		return err
	}
	m, err := cache.Data(ctx)
	if err != nil {
		return err
	}
	return storeFile(ctx, m)
}

func Remove(ctx context.Context, key any) error {
	val, err := cache.Remove(ctx, key)
	if err != nil {
		return err
	}
	if val.IsEmpty() {
		return nil
	}
	m, err := cache.Data(ctx)
	if err != nil {
		return err
	}
	return storeFile(ctx, m)
}
