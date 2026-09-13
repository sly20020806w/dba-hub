package web

import "embed"

// Assets 内嵌 Web 控制台
//
//go:embed all:web
var Assets embed.FS
