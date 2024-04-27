package consts

import (
	"github.com/gogf/gf/v2"
	"runtime"
)

const (
	ProjName = "count"
	Version  = "0.1.0"
)

var (
	GitTag      = ""
	GitCommit   = ""
	BuildTime   = ""
	Description = "Version: " + Version +
		"\nGo Version: " + runtime.Version() +
		"\nGoFrame Version: " + gf.VERSION +
		"\nGit Tag: " + GitTag +
		"\nGit Commit: " + GitCommit +
		"\nBuild Time: " + BuildTime
)
