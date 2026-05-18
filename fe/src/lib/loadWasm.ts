declare global {
  interface Window {
    Go: any;

    goInitRenderer?: (canvasId: string) => string;

    goHandleMouseMove?: (deltaX: number, deltaY: number) => void;
  }
}

let wasmLoaded = false;

let wasmLoading: Promise<void> | null = null;

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

    const go = new window.Go();

    const response = await fetch("/wasm/main.wasm");

    if (!response.ok) {
      throw new Error("Failed to fetch main.wasm");
    }

    const result = await WebAssembly.instantiateStreaming(
      response,
      go.importObject,
    );

    // Start Go runtime
    go.run(result.instance);

    wasmLoaded = true;

    console.log("Go WASM initialized");
  })();

  return wasmLoading;
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
