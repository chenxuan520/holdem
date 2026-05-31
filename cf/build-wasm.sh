#!/usr/bin/env bash
# Build the Go referee core to WebAssembly (full Go toolchain, GOOS=js) and
# stage it next to the Worker entry along with the matching wasm_exec.js glue.
#
# Full Go (not TinyGo) is intentional: the prompt-cache feature depends on
# byte-exact encoding/json field ordering + omitempty, which is TinyGo's weak
# spot. M0 measured the full-Go module at ~1.38MB gzip (well under the 3MB free
# Worker limit) and ~8ms cold instantiate (under the 10ms free CPU budget),
# with sub-ms warm reducer calls, so we keep full Go for maximum fidelity.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND="$HERE/../backend"
OUT="$HERE/src/core.wasm"
GOROOT="$(go env GOROOT)"

(cd "$BACKEND" && GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o "$OUT" ./cmd/wasmcore)
cp "$GOROOT/lib/wasm/wasm_exec.js" "$HERE/src/wasm_exec.js"

bytes="$(wc -c < "$OUT" | tr -d ' ')"
echo "built $OUT (${bytes} bytes) + src/wasm_exec.js"
