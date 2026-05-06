// Package web embeds the html templates and static assets into the binary.
package web

import "embed"

// FS holds the embedded templates and static assets.
//
//go:embed templates/*.html static/*
var FS embed.FS
