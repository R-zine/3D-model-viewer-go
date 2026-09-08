interface GoRuntime {
  importObject: WebAssembly.Imports;
  run(instance: WebAssembly.Instance): Promise<void>;
}

interface GoConstructor {
  new (): GoRuntime;
}

declare global {
  interface Window {
    Go?: GoConstructor;
    goInitRenderer?: (canvasId: string) => string;
    goRotateRenderer?: (deltaX: number, deltaY: number) => string;
    goLoadModel?: (data: Uint8Array) => string;
    goRendererFPS?: () => number;
    goDisposeRenderer?: () => void;
  }
}

export const MAX_MODEL_BYTES = 128 * 1024 * 1024;

let wasmLoaded = false;
let wasmLoading: Promise<void> | null = null;
let goInstance: GoRuntime | null = null;

export function loadWasm(): Promise<void> {
  if (wasmLoaded) {
    return Promise.resolve();
  }
  if (wasmLoading) {
    return wasmLoading;
  }
  wasmLoading = initializeWasm().catch((error: unknown) => {
    wasmLoading = null;
    goInstance = null;
    throw error;
  });
  return wasmLoading;
}

async function initializeWasm(): Promise<void> {
  await loadScript(assetPath("wasm/wasm_exec.js"));
  if (!window.Go) {
    throw new Error("Go WebAssembly runtime is unavailable");
  }
  goInstance = new window.Go();
  const response = await fetch(assetPath("wasm/main.wasm"));
  if (!response.ok) {
    throw new Error(`Failed to fetch main.wasm (${response.status})`);
  }

  const fallbackResponse = response.clone();
  let result: WebAssembly.WebAssemblyInstantiatedSource;
  try {
    result = await WebAssembly.instantiateStreaming(response, goInstance.importObject);
  } catch (streamingError: unknown) {
    try {
      result = await WebAssembly.instantiate(
        await fallbackResponse.arrayBuffer(),
        goInstance.importObject,
      );
    } catch {
      throw streamingError;
    }
  }

  void goInstance.run(result.instance).catch((error: unknown) => {
    wasmLoaded = false;
    wasmLoading = null;
    document.dispatchEvent(new CustomEvent("renderer-error", {
      detail: `Go WebAssembly runtime stopped: ${errorMessage(error)}`,
    }));
  });
  if (!window.goInitRenderer || !window.goLoadModel) {
    throw new Error("Go WebAssembly callbacks were not registered");
  }
  wasmLoaded = true;
}

export async function initRenderer(canvasId: string): Promise<void> {
  await loadWasm();
  if (!window.goInitRenderer) {
    throw new Error("goInitRenderer is unavailable");
  }
  throwIfGoError(window.goInitRenderer(canvasId));
}

export async function loadModel(file: File): Promise<void> {
  loadModelBytes(await readModelFile(file));
}

export async function readModelFile(file: File): Promise<Uint8Array> {
  if (!file.name.toLowerCase().endsWith(".glb")) {
    throw new Error("Please select a .glb file");
  }
  if (file.size <= 0) {
    throw new Error("The selected model is empty");
  }
  if (file.size > MAX_MODEL_BYTES) {
    throw new Error(`The selected model exceeds the ${MAX_MODEL_BYTES >> 20} MiB limit`);
  }
  return new Uint8Array(await file.arrayBuffer());
}

export function loadModelBytes(data: Uint8Array): void {
  if (!window.goLoadModel) {
    throw new Error("The renderer is not initialized");
  }
  if (data.byteLength <= 0 || data.byteLength > MAX_MODEL_BYTES) {
    throw new Error("Model data has an invalid size");
  }
  throwIfGoError(window.goLoadModel(data));
}

export function rotateRenderer(deltaX: number, deltaY: number): void {
  if (window.goRotateRenderer) {
    throwIfGoError(window.goRotateRenderer(deltaX, deltaY));
  }
}

export function rendererFPS(): number {
  return window.goRendererFPS?.() ?? 0;
}

export function disposeRenderer(): void {
  window.goDisposeRenderer?.();
}

function throwIfGoError(message: string): void {
  if (message) {
    throw new Error(message);
  }
}

function loadScript(src: string): Promise<void> {
  return new Promise<void>((resolve, reject) => {
    const existing = document.querySelector<HTMLScriptElement>(`script[src="${src}"]`);
    if (existing) {
      if (window.Go) {
        resolve();
        return;
      }
      if (existing.dataset.loadState === "loading") {
        existing.addEventListener("load", () => resolve(), { once: true });
        existing.addEventListener("error", () => reject(new Error(`Failed to load ${src}`)), { once: true });
        return;
      }
      existing.remove();
    }

    const script = document.createElement("script");
    script.src = src;
    script.async = true;
    script.dataset.loadState = "loading";
    script.addEventListener("load", () => {
      script.dataset.loadState = "loaded";
      resolve();
    }, { once: true });
    script.addEventListener("error", () => {
      script.remove();
      reject(new Error(`Failed to load ${src}`));
    }, { once: true });
    document.body.appendChild(script);
  });
}

function assetPath(relativePath: string): string {
  const base = import.meta.env.BASE_URL.endsWith("/")
    ? import.meta.env.BASE_URL
    : `${import.meta.env.BASE_URL}/`;
  return `${base}${relativePath.replace(/^\//, "")}`;
}

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
