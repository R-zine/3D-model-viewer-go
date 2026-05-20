# Go WASM 3D Model Viewer

A lightweight 3D model viewer built with:

- Go
- WebAssembly (WASM)
- WebGL2
- React
- TypeScript
- Material UI (MUI)

The renderer is written in Go and compiled to WASM, while the frontend UI is built with React.

---

# Features

- GLB model loading
- WebGL2 rendering
- Texture support
- Basic lighting
- Mouse rotation controls
- FPS counter
- React + MUI UI
- WASM renderer integration

---

# Project Structure

```text
.
├── fe/                 # React frontend
├── go-wasm/            # Go WASM renderer
├── scripts/            # Build scripts
└── README.md
```

---

# Requirements

Make sure you have installed:

- Go
- Node.js
- npm

---

# Available Scripts

Run these commands from the root of the repository.

## Build the WASM module

```bash
npm run build:wasm
```

This compiles the Go renderer into WebAssembly.

---

## Start the frontend

```bash
npm run fe
```

This installs frontend dependencies and starts the React development server.

---

# Development Workflow

1. Build the WASM module:

```bash
npm run build:wasm
```

2. Start the frontend:

```bash
npm run fe
```

3. Open the local development URL shown in the terminal.

---

# Controls

| Action        | Description             |
| ------------- | ----------------------- |
| Mouse Move    | Rotate the model        |
| Upload Button | Load a custom GLB model |

---

# Notes

- The renderer currently supports `.glb` files.
- Rendering uses WebGL2.
- The project is intended as a lightweight experimental viewer and rendering playground.

---

# License

MIT
