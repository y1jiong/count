package cfg

import "context"

const (
	pathServerAddress = "server.address"
)

func GetServerAddress(ctx context.Context) string {
	return get(ctx, pathServerAddress, ":8080").String()
}
