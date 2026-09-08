import { spawnSync } from "node:child_process";
import { existsSync } from "node:fs";
import path from "node:path";
import process from "node:process";
import { fileURLToPath } from "node:url";

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(scriptDirectory, "..");
const goDirectory = path.join(root, "go-wasm");
const frontendDirectory = path.join(root, "fe");

const npmCliCandidates = [
  process.env.npm_execpath,
  path.join(path.dirname(process.execPath), "node_modules", "npm", "bin", "npm-cli.js"),
].filter(Boolean);
const npmCli = npmCliCandidates.find((candidate) => existsSync(candidate));
const npmCommand = process.platform === "win32" ? process.execPath : "npm";
const npmArguments = process.platform === "win32" ? [npmCli] : [];

function fail(message) {
  console.error(`\nPre-commit verification failed: ${message}`);
  process.exit(1);
}

function run(label, command, arguments_, cwd = root) {
  console.log(`\n[pre-commit] ${label}`);
  const result = spawnSync(command, arguments_, {
    cwd,
    env: process.env,
    stdio: "inherit",
    shell: false,
  });

  if (result.error) {
    fail(`${command} could not be started: ${result.error.message}`);
  }
  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}

function checkGoFormatting() {
  console.log("\n[pre-commit] Go formatting");
  const result = spawnSync("gofmt", ["-l", "."], {
    cwd: goDirectory,
    encoding: "utf8",
    shell: false,
  });

  if (result.error) {
    fail(`gofmt could not be started: ${result.error.message}`);
  }
  if (result.status !== 0) {
    process.stderr.write(result.stderr ?? "");
    process.exit(result.status ?? 1);
  }

  const unformattedFiles = result.stdout.trim();
  if (unformattedFiles) {
    console.error("These Go files need gofmt:");
    console.error(unformattedFiles);
    process.exit(1);
  }
}

if (!existsSync(path.join(frontendDirectory, "node_modules"))) {
  fail("frontend dependencies are missing; run `npm --prefix fe ci`");
}
if (process.platform === "win32" && !npmCli) {
  fail("npm-cli.js could not be found next to Node; ensure npm is installed and available");
}

run("Staged whitespace and conflict-marker check", "git", ["diff", "--cached", "--check"]);
checkGoFormatting();
run("Go module consistency", "go", ["mod", "tidy", "-diff"], goDirectory);
run("Go tests", "go", ["test", "./..."], goDirectory);
run("Go static analysis", "go", ["vet", "./..."], goDirectory);
run("TypeScript type-check", npmCommand, [...npmArguments, "run", "typecheck"], frontendDirectory);
run("Frontend lint", npmCommand, [...npmArguments, "run", "lint"], frontendDirectory);
run("Production and WebAssembly build", npmCommand, [...npmArguments, "run", "build"], frontendDirectory);

console.log("\nPre-commit verification passed.");
