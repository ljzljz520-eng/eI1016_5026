package webui

import "embed"

//go:embed src/*.html src/*.css src/*.js
var Files embed.FS
