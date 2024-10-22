package cfg

import "context"

const (
	pathCacheFile = "cache.file"
)

func GetCacheFilePath(ctx context.Context) string {
	return get(ctx, pathCacheFile, "cache.json").String()
}
