//go:build !js || !wasm

// Host stub: main.go imports syscall/js and only compiles for GOOS=js
// GOARCH=wasm. This stub keeps `go build ./...` / `go test ./...` happy on the
// host toolchain by giving package main a buildable file for every other
// target. The wasm core is only ever built with GOOS=js GOARCH=wasm.
package main

func main() {}
