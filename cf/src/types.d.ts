// Ambient declarations for build-time assets that have no .d.ts of their own.

// `import core from "./core.wasm"` yields a precompiled module (wrangler
// CompiledWasm rule); we instantiate it per isolate in wasm.ts.
declare module "*.wasm" {
  const mod: WebAssembly.Module;
  export default mod;
}

// Go's wasm_exec.js is untyped glue imported for its side effect: it assigns
// `globalThis.Go`. We never import a binding from it.
declare module "*/wasm_exec.js";
