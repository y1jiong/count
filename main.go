package main

import (
	"context"
	"count/internal/consts"
	"count/internal/controller"
	"count/internal/service/install"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
	flag "github.com/spf13/pflag"
	"net/http"
)

var (
	versionFlag   = flag.BoolP("version", "V", false, "查看当前版本后退出")
	installFlag   = flag.BoolP("install", "I", false, "安装服务并退出")
	uninstallFlag = flag.BoolP("uninstall", "U", false, "卸载服务并退出")
)

func main() {
	ctx := context.Background()
	exit, err := doFlag(ctx)
	if err != nil {
		g.Log().Error(ctx, err)
		return
	}
	if exit {
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /count/{$}", controller.Count)

	address, err := gcfg.Instance().Get(ctx, "server.address", ":8080")
	if err != nil {
		g.Log().Error(ctx, err)
		return
	}

	g.Log().Info(ctx, "Server running on", address.String())
	// Route
	g.Log().Info(ctx, "POST /count/{$}")

	if err = http.ListenAndServe(address.String(), mux); err != nil {
		g.Log().Error(ctx, err)
		return
	}
}

func doFlag(ctx context.Context) (exit bool, err error) {
	flag.Parse()
	if *versionFlag {
		consts.PrintVersion()
		return true, nil
	}
	if *installFlag {
		return true, install.Install(ctx)
	}
	if *uninstallFlag {
		return true, install.Uninstall(ctx)
	}
	return
}
