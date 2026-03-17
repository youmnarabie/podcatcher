// Package web exports the embedded frontend build output.
package web

import "embed"

//go:embed dist
var Dist embed.FS
