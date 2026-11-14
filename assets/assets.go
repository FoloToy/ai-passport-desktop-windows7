// Package assets embeds the app icon.
package assets

import _ "embed"

//go:embed icon.png
var Icon []byte
