// Package web embeds the reviewed production dashboard assets.
package web

import "embed"

// Assets contains only files produced in web/dist before the Go build.
//
//go:embed all:dist
var Assets embed.FS
