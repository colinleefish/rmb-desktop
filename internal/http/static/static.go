package static

import "embed"

// Web holds the Vite production build copied into web/ by `make ui-build`.
//
//go:embed all:web
var Web embed.FS
