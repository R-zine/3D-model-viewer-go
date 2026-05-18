import { execSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";

const root = process.cwd();

const goDir = path.join(root, "go-wasm");
const goWasmDir = path.join(goDir, "wasm");

const webWasmDir = path.join(root, "fe", "public", "wasm");

fs.mkdirSync(goWasmDir, { recursive: true });
fs.mkdirSync(webWasmDir, { recursive: true });

console.log("Building Go WASM...");

execSync("go build -o wasm/main.wasm", {
  cwd: goDir,
  env: {
    ...process.env,
    GOOS: "js",
    GOARCH: "wasm",
  },
  stdio: "inherit",
});

console.log("Copying wasm_exec.js...");

const goroot = execSync("go env GOROOT").toString().trim();

const wasmExecCandidates = [
  path.join(goroot, "lib", "wasm", "wasm_exec.js"),
  path.join(goroot, "misc", "wasm", "wasm_exec.js"),
];

const wasmExecPath = wasmExecCandidates.find(fs.existsSync);

if (!wasmExecPath) {
  throw new Error("Could not locate wasm_exec.js");
}

fs.copyFileSync(wasmExecPath, path.join(goWasmDir, "wasm_exec.js"));

console.log("Copying files to React public folder...");

fs.copyFileSync(
  path.join(goWasmDir, "main.wasm"),
  path.join(webWasmDir, "main.wasm"),
);

fs.copyFileSync(
  path.join(goWasmDir, "wasm_exec.js"),
  path.join(webWasmDir, "wasm_exec.js"),
);

console.log("WASM build complete");
