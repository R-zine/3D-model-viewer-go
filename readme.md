# Go WASM 3D Model Viewer

A small GLB 2.0 renderer written in Go, compiled to WebAssembly, and presented through a React UI. The renderer and GLB parser use only the Go standard library and browser WebGL2 APIs; React and Material UI are used for the surrounding interface.

This is an educational renderer rather than a complete implementation of every GLTF extension. Unsupported model features are rejected with an actionable error instead of being silently misrendered.

## Features

- Strict, allocation-bounded GLB 2.0 parsing
- Multiple meshes, selected scenes, node hierarchies, and matrix/TRS transforms
- Indexed and non-indexed primitives in all core GLTF primitive modes
- Float positions; float or normalized integer normals and texture coordinates
- Embedded PNG, JPEG, and WebP base-color textures
- Base-color factors, alpha mask/blend modes, and double-sided materials
- Generated normals and a white fallback texture when optional data is absent
- Model bounds-based centering/scaling, responsive high-DPI rendering, and drag rotation
- WebGL context-loss recovery and deterministic GPU resource disposal
- Host-side parser tests/fuzz seeds and Playwright browser smoke tests

## Requirements

- Go 1.26.1 or newer
- Node.js 22.12 or newer
- npm
- A browser with WebGL2

## Setup and development

```bash
npm --prefix fe ci
npm run fe
```

Open the URL printed by Vite. A small built-in cube is generated in TypeScript, so the repository does not require a large binary demo asset.
The frontend's development and production-build commands compile the Go module and copy the matching Go runtime before Vite starts, so a separate WASM build step is not required.

For a production build:

```bash
npm run build
```

`build:wasm` remains available when only the WebAssembly assets are needed. It writes the optimized module and the matching Go runtime directly to `fe/public/wasm`. These generated files are intentionally ignored by Git.

## Tests

```bash
cd go-wasm
go test ./...
go test -cover ./...
go vet ./...
go test ./gltf -run='^$' -fuzz='^FuzzParseGLB$' -fuzztime=3s
go test ./gltf -run='^$' -fuzz='^FuzzParseGLBJSON$' -fuzztime=3s

cd ../fe
npm run lint
npm run build
npx playwright install chromium
npm run test:e2e
```

From the repository root, `npm run test:all` performs the complete build and test sequence. The CI workflow enforces formatting, vetting, race-enabled Go tests, at least 90% coverage of host-testable Go code, two fuzz targets, host and WASM compilation, frontend type/lint/build checks, and ten serial Chromium integration scenarios. Browser tests run serially because every page owns both a Go runtime and a WebGL2 context.

### Pre-commit verification

Install the repository-managed hook once per clone:

```bash
npm run hooks:install
```

Before each commit, it checks staged whitespace and conflict markers, Go formatting and module consistency, Go tests and vet, TypeScript types, ESLint, and a complete frontend/WebAssembly production build. Run the same checks manually with `npm run verify:commit`. Fuzzing, race-enabled tests, coverage enforcement, and browser scenarios remain in `npm run test:all` and CI so routine commits stay deterministic and reasonably fast.

To validate a model against the renderer's supported subset without opening a browser:

```bash
cd go-wasm
go run ./cmd/gltfcheck ../path/to/model.glb
```

## Controls

| Action | Result |
| --- | --- |
| Pointer drag | Rotate the model |
| Upload GLB | Validate and replace the current model |

Uploaded files are limited to 128 MiB. A failed upload leaves the current model intact and displays the parser or renderer error in the UI.

## Architecture

```text
go-wasm/gltf       Pure Go GLB container, schema, accessor, scene, and material parsing
go-wasm/renderer   WebGL2 resource ownership, shaders, textures, and render loop
go-wasm/main.go    Small typed JavaScript/WebAssembly callback boundary
fe/src/lib         WASM bootstrap and generated demo model
fe/src/App.tsx     Responsive UI, upload lifecycle, resize, and pointer controls
```

The parser is deliberately independent of `syscall/js`. This keeps untrusted binary validation testable on the host and prevents malformed data from reaching GPU upload code.

## Supported GLTF subset and limitations

The loader accepts GLB version 2 with a JSON chunk and optional embedded BIN chunk. Core node transforms, scenes, mesh primitives, accessors, and base-color material properties are supported.

The following are currently rejected or not rendered:

- External buffers or external image URLs
- Sparse accessors
- Skins, morph targets, and animations
- Draco, Meshopt, KTX2, and other compression/texture extensions
- Normal, occlusion, emissive, metallic, and roughness shading inputs
- Cameras and punctual lights from the file

Transparent primitives are blended but not depth-sorted, so overlapping blend materials can still show ordering artifacts. Add features alongside conformance fixtures rather than silently accepting metadata the renderer ignores.

## License

[MIT](LICENSE)
