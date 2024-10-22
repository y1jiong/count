package cache

import (
	"context"
	"count/internal/cfg"
	"encoding/json"
	"github.com/gogf/gf/v2/util/gconv"
	"os"
)

func loadFile(ctx context.Context) map[any]any {
	content, err := os.ReadFile(cfg.GetCacheFilePath(ctx))
	if err != nil {
		return nil
	}
	var m map[string]any
	if err = json.Unmarshal(content, &m); err != nil {
		return nil
	}
	var mm map[any]any
	if err = gconv.Scan(m, &mm); err != nil {
		return nil
	}
	return mm
}

func storeFile(ctx context.Context, m map[any]any) error {
	content, err := json.Marshal(gconv.Map(m))
	if err != nil {
		return err
	}
	return os.WriteFile(cfg.GetCacheFilePath(ctx), content, 0644)
}
