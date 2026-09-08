import { execFileSync } from "node:child_process";
import { chmodSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const scriptDirectory = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(scriptDirectory, "..");
const hook = path.join(root, ".githooks", "pre-commit");
const safeRoot = root.replaceAll("\\", "/");

// Git ignores hooks without the executable bit on POSIX systems. chmod is
// harmless on Windows and keeps this installer portable across environments.
chmodSync(hook, 0o755);

const gitArguments = ["-c", `safe.directory=${safeRoot}`];
execFileSync(
  "git",
  [...gitArguments, "config", "--local", "core.hooksPath", ".githooks"],
  { cwd: root, stdio: "inherit" },
);

const configuredPath = execFileSync(
  "git",
  [...gitArguments, "config", "--local", "--get", "core.hooksPath"],
  { cwd: root, encoding: "utf8" },
).trim();

if (configuredPath !== ".githooks") {
  throw new Error(`Unexpected core.hooksPath value: ${configuredPath}`);
}

console.log("Installed repository hooks from .githooks");
