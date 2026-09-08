import { Buffer } from "node:buffer";
import { expect, test, type Locator, type Page } from "@playwright/test";
import { demoModelBuffer, jsonGLB } from "./fixtures";

interface RendererAPIWindow extends Window {
  goInitRenderer?: (...args: unknown[]) => string;
  goRotateRenderer?: (...args: unknown[]) => string;
  goLoadModel?: (...args: unknown[]) => string;
  goRendererFPS?: (...args: unknown[]) => number;
  goDisposeRenderer?: (...args: unknown[]) => void;
  testLoseContext?: WEBGL_lose_context;
}

test("initializes WebGL, reports FPS, resizes, and rotates the model", async ({ page }) => {
  const pageErrors: string[] = [];
  page.on("pageerror", (error) => pageErrors.push(error.message));
  await openReadyViewer(page);

  const canvas = page.getByTestId("viewer");
  await expect(canvas).toBeVisible();
  await expect.poll(() => rendererFPS(page)).toBeGreaterThan(0);
  await expect.poll(() => backingSizeMatchesCSS(canvas)).toBe(true);

  await page.setViewportSize({ width: 900, height: 700 });
  await expect.poll(() => backingSizeMatchesCSS(canvas)).toBe(true);

  const before = await canvas.screenshot();
  const bounds = await canvas.boundingBox();
  if (!bounds) {
    throw new Error("Canvas has no bounding box");
  }
  await page.mouse.move(bounds.x + bounds.width / 2, bounds.y + bounds.height / 2);
  await page.mouse.down();
  await expect(canvas).toHaveCSS("cursor", "grabbing");
  await page.mouse.move(bounds.x + bounds.width * 0.75, bounds.y + bounds.height * 0.65);
  await page.mouse.up();
  await expect(canvas).toHaveCSS("cursor", "grab");
  await expect.poll(async () => !(await canvas.screenshot()).equals(before)).toBe(true);
  expect(pageErrors).toEqual([]);
});

test("loads a valid uploaded GLB and accepts a case-insensitive extension", async ({ page }) => {
  await openReadyViewer(page);
  await upload(page, "replacement.GLB", demoModelBuffer());
  await expect(page.getByText("replacement.GLB", { exact: true })).toBeVisible();
  await expect(page.getByRole("alert")).toHaveCount(0);
  await expect(page.getByLabel("Loading model")).toHaveCount(0);
  await expect.poll(() => rendererFPS(page)).toBeGreaterThan(0);
});

test("rejects an invalid extension before invoking the parser", async ({ page }) => {
  await openReadyViewer(page);
  await upload(page, "model.gltf", demoModelBuffer(), "model/gltf+json");
  await expect(page.getByRole("alert")).toContainText("Please select a .glb file");
  await expect(page.getByText("Built-in cube", { exact: true })).toBeVisible();
});

test("rejects an empty upload without replacing the current model", async ({ page }) => {
  await openReadyViewer(page);
  await upload(page, "empty.glb", Buffer.alloc(0));
  await expect(page.getByRole("alert")).toContainText("selected model is empty");
  await expect(page.getByText("Built-in cube", { exact: true })).toBeVisible();
});

test("rejects an oversized upload before reading its contents", async ({ page }) => {
  await openReadyViewer(page);
  await page.locator('input[type="file"]').evaluate((element: HTMLInputElement) => {
    const oversized = new File([new Uint8Array()], "oversized.glb", { type: "model/gltf-binary" });
    Object.defineProperty(oversized, "size", { value: 129 * 1024 * 1024 });
    const transfer = new DataTransfer();
    transfer.items.add(oversized);
    element.files = transfer.files;
    element.dispatchEvent(new Event("change", { bubbles: true }));
  });
  await expect(page.getByRole("alert")).toContainText("exceeds the 128 MiB limit");
  await expect(page.getByText("Built-in cube", { exact: true })).toBeVisible();
});

test("reports malformed GLB data without replacing the current model", async ({ page }) => {
  await openReadyViewer(page);
  await upload(page, "broken.glb", Buffer.from("not a glb"));
  await expect(page.getByRole("alert")).toContainText("GLB header is truncated");
  await expect(page.getByText("Built-in cube", { exact: true })).toBeVisible();
});

test("reports unsupported required GLTF extensions", async ({ page }) => {
  await openReadyViewer(page);
  const model = jsonGLB({
    asset: { version: "2.0" },
    extensionsRequired: ["KHR_draco_mesh_compression"],
  });
  await upload(page, "compressed.glb", model);
  await expect(page.getByRole("alert")).toContainText("required GLTF extensions are unsupported");
  await expect(page.getByText("Built-in cube", { exact: true })).toBeVisible();
});

test("validates the exported WASM API boundary", async ({ page }) => {
  await openReadyViewer(page);
  const errors = await page.evaluate(() => {
    const api = window as RendererAPIWindow;
    return {
      missingCanvasID: api.goInitRenderer?.(),
      nonFiniteRotation: api.goRotateRenderer?.(Number.NaN, 0),
      wrongModelType: api.goLoadModel?.("not bytes"),
      emptyModel: api.goLoadModel?.(new Uint8Array()),
    };
  });
  expect(errors).toEqual({
    missingCanvasID: "a canvas id is required",
    nonFiniteRotation: "pointer deltas must be finite",
    wrongModelType: "model data must be a Uint8Array",
    emptyModel: "model data is empty",
  });
});

test("disposes and reinitializes the renderer deterministically", async ({ page }) => {
  await openReadyViewer(page);
  const bytes = [...demoModelBuffer()];
  const results = await page.evaluate((modelBytes) => {
    const api = window as RendererAPIWindow;
    api.goDisposeRenderer?.();
    const fpsAfterDispose = api.goRendererFPS?.() ?? -1;
    const missingCanvas = api.goInitRenderer?.("missing-canvas");
    const initialize = api.goInitRenderer?.("viewer");
    const load = api.goLoadModel?.(new Uint8Array(modelBytes));
    return { fpsAfterDispose, missingCanvas, initialize, load };
  }, bytes);
  expect(results).toEqual({
    fpsAfterDispose: 0,
    missingCanvas: 'canvas "missing-canvas" was not found',
    initialize: "",
    load: "",
  });
  await expect.poll(() => rendererFPS(page)).toBeGreaterThan(0);
});

test("recovers the model after WebGL context loss", async ({ page }) => {
  await openReadyViewer(page);
  const supported = await page.evaluate(() => {
    const canvas = document.querySelector<HTMLCanvasElement>("#viewer");
    const extension = canvas?.getContext("webgl2")?.getExtension("WEBGL_lose_context") ?? null;
    (window as RendererAPIWindow).testLoseContext = extension ?? undefined;
    return extension !== null;
  });
  test.skip(!supported, "WEBGL_lose_context is unavailable in this browser");

  await page.evaluate(() => (window as RendererAPIWindow).testLoseContext?.loseContext());
  await expect(page.getByRole("alert")).toContainText("WebGL context was lost");
  await expect.poll(() => rendererFPS(page)).toBe(0);
  await page.evaluate(() => (window as RendererAPIWindow).testLoseContext?.restoreContext());
  await expect.poll(() => rendererFPS(page), { timeout: 10_000 }).toBeGreaterThan(0);
});

async function openReadyViewer(page: Page): Promise<void> {
  await page.goto("/");
  await expect(page.getByText("Ready", { exact: true })).toBeVisible();
  await expect(page.getByText("Built-in cube", { exact: true })).toBeVisible();
  await expect(page.getByRole("alert")).toHaveCount(0);
}

async function upload(page: Page, name: string, buffer: Buffer, mimeType = "model/gltf-binary"): Promise<void> {
  await page.locator('input[type="file"]').setInputFiles({ name, mimeType, buffer });
}

async function rendererFPS(page: Page): Promise<number> {
  return page.evaluate(() => (window as RendererAPIWindow).goRendererFPS?.() ?? 0);
}

async function backingSizeMatchesCSS(canvas: Locator): Promise<boolean> {
  return canvas.evaluate((element: HTMLCanvasElement) => {
    const bounds = element.getBoundingClientRect();
    const ratio = Math.min(window.devicePixelRatio || 1, 2);
    return Math.abs(element.width - Math.round(bounds.width * ratio)) <= 1
      && Math.abs(element.height - Math.round(bounds.height * ratio)) <= 1;
  });
}
