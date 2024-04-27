package cache

import (
	"context"
	"encoding/json"
	"github.com/gogf/gf/v2/os/gcfg"
	"github.com/gogf/gf/v2/util/gconv"
	"os"
)

func loadFile(ctx context.Context) map[any]any {
	val, err := gcfg.Instance().Get(ctx, "cache.file", "cache.json")
	if err != nil {
		return nil
	}
	content, err := os.ReadFile(val.String())
	if err != nil {
		return nil
	}
	var m map[string]any
	err = json.Unmarshal(content, &m)
	var mm map[any]any
	err = gconv.Scan(m, &mm)
	if err != nil {
		return nil
	}
	return mm
}

func storeFile(ctx context.Context, m map[any]any) error {
	val, err := gcfg.Instance().Get(ctx, "cache.file", "cache.json")
	if err != nil {
		return err
	}
	content, err := json.Marshal(gconv.Map(m))
	if err != nil {
		return err
	}
	return os.WriteFile(val.String(), content, 0644)
}
