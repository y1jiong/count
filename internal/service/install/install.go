package install

import (
	"context"
	"count/internal/consts"
	"errors"
	"os"
	"runtime"

	"github.com/gogf/gf/v2/frame/g"
)

const (
	installPath = "/etc/systemd/system/" + consts.ProjName + ".service"
)

func isWindows() bool {
	return runtime.GOOS == "windows"
}

func Install(ctx context.Context) (err error) {
	if isWindows() {
		return errors.New("windows 暂不支持安装到系统")
	}

	wd, err := os.Getwd()
	if err != nil {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}

	serviceContent := []byte(
		"[Unit]\n" +
			"Description=" + consts.ProjName + " Service\n" +
			"Wants=network-online.target\n" +
			"After=network-online.target\n\n" +
			"[Service]\n" +
			"Type=simple\n" +
			"WorkingDirectory=" + wd +
			"\nExecStart=" + exe +
			"\nRestart=on-failure\n" +
			"RestartSec=2\n" +
			"LimitNOFILE=65535\n\n" +
			"[Install]\n" +
			"WantedBy=multi-user.target\n",
	)
	if err = os.WriteFile(installPath, serviceContent, 0o644); err != nil {
		return
	}
	g.Log().Notice(ctx, "安装服务成功\n可以使用 systemctl 管理", consts.ProjName, "服务了")
	return
}

func Uninstall(ctx context.Context) (err error) {
	if isWindows() {
		return errors.New("windows 暂不支持安装到系统")
	}

	if err = os.Remove(installPath); err != nil {
		return
	}
	g.Log().Notice(ctx, "卸载服务成功")
	return
}
