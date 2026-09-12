// Package nanotube provê os assets estáticos embutidos do frontend SPA.
package nanotube

import "embed"

//go:embed all:frontend/dist
var FrontendAssets embed.FS
