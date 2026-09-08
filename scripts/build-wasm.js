import { execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(scriptDirectory, "..");
const goDirectory = path.join(root, "go-wasm");
const outputDirectory = path.join(root, "fe", "public", "wasm");
const wasmOutput = path.join(outputDirectory, "main.wasm");

fs.mkdirSync(outputDirectory, { recursive: true });

console.log("Building optimized Go WebAssembly module...");
execFileSync(
  "go",
  ["build", "-buildvcs=false", "-trimpath", "-ldflags=-s -w", "-o", wasmOutput, "."],
  {
    cwd: goDirectory,
    env: { ...process.env, GOOS: "js", GOARCH: "wasm" },
    stdio: "inherit",
  },
);

const goRoot = execFileSync("go", ["env", "GOROOT"], { encoding: "utf8" }).trim();
const runtimeCandidates = [
  path.join(goRoot, "lib", "wasm", "wasm_exec.js"),
  path.join(goRoot, "misc", "wasm", "wasm_exec.js"),
];
const runtime = runtimeCandidates.find((candidate) => fs.existsSync(candidate));
if (!runtime) {
  throw new Error(`Could not locate wasm_exec.js under ${goRoot}`);
}
fs.copyFileSync(runtime, path.join(outputDirectory, "wasm_exec.js"));
console.log(`WASM build complete: ${path.relative(root, wasmOutput)}`);
