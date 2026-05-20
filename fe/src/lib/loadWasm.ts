declare global {
  interface Window {
    Go: any;

    goInitRenderer?: (canvasId: string) => string;

    goHandleMouseMove?: (deltaX: number, deltaY: number) => void;

    goLoadModel?: (data: Uint8Array) => void;
  }
}

let wasmLoaded = false;

let wasmLoading: Promise<void> | null = null;

let goInstance: any = null;

export async function loadWasm() {
  // Already initialized
  if (wasmLoaded) {
    return;
  }

  // Prevent duplicate loads
  if (wasmLoading) {
    return wasmLoading;
  }

  wasmLoading = (async () => {
    // Load Go runtime
    await loadScript("/wasm/wasm_exec.js");

    if (!window.Go) {
      throw new Error("Go runtime unavailable");
    }

    goInstance = new window.Go();

    const response = await fetch("/wasm/main.wasm");

    if (!response.ok) {
      throw new Error("Failed to fetch main.wasm");
    }

    let result: WebAssembly.WebAssemblyInstantiatedSource;

    // Prefer instantiateStreaming
    if ("instantiateStreaming" in WebAssembly) {
      result = await WebAssembly.instantiateStreaming(
        response,
        goInstance.importObject,
      );
    } else {
      const bytes = await response.arrayBuffer();

      result = await WebAssembly.instantiate(bytes, goInstance.importObject);
    }

    // Start Go runtime
    goInstance.run(result.instance);

    wasmLoaded = true;

    console.log("Go WASM initialized");
  })();

  return wasmLoading;
}

export async function initRenderer(canvasId: string) {
  await loadWasm();

  if (!window.goInitRenderer) {
    throw new Error("goInitRenderer not found");
  }

  return window.goInitRenderer(canvasId);
}

export async function loadModel(file: File) {
  await loadWasm();

  if (!window.goLoadModel) {
    throw new Error("goLoadModel not found");
  }

  const buffer = await file.arrayBuffer();

  const uint8 = new Uint8Array(buffer);

  window.goLoadModel(uint8);
}

function loadScript(src: string) {
  return new Promise<void>((resolve, reject) => {
    // Already loaded
    const existing = document.querySelector(`script[src="${src}"]`);

    if (existing) {
      resolve();
      return;
    }

    const script = document.createElement("script");

    script.src = src;

    script.async = true;

    script.onload = () => resolve();

    script.onerror = () => reject(new Error(`Failed to load ${src}`));

    document.body.appendChild(script);
  });
}
