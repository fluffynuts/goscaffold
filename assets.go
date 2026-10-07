// Package goscaffold holds what goscaffold writes into the projects it
// scaffolds: the templates, and this repository's own .gitignore, which new
// projects start from. go:embed can't reach above a package's own folder,
// which is why they are embedded from here rather than from next to the code
// that uses them.
package goscaffold

import "embed"

// Assets is the embedded templates folder and .gitignore.
//
//go:embed all:templates .gitignore
var Assets embed.FS
