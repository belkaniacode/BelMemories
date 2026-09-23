//go:build !production

package main

// devBuild is true for `wails dev` and plain `go build`/`go test`
// (`wails build` adds the "production" tag).
const devBuild = true
