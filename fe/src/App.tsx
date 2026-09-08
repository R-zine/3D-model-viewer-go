import { useEffect, useRef, useState, type ChangeEvent, type PointerEvent } from "react";
import {
  Alert,
  AppBar,
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  Container,
  Divider,
  LinearProgress,
  Stack,
  Toolbar,
  Typography,
} from "@mui/material";
import {
  CloudUpload as CloudUploadIcon,
  Speed as SpeedIcon,
  ViewInAr as ViewInArIcon,
} from "@mui/icons-material";
import { createDemoModel } from "./lib/demoModel";
import {
  disposeRenderer,
  initRenderer,
  loadModelBytes,
  readModelFile,
  rendererFPS,
  rotateRenderer,
} from "./lib/loadWasm";

interface PointerPosition {
  id: number;
  x: number;
  y: number;
}

export default function App() {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const pointer = useRef<PointerPosition | null>(null);
  const loadRequest = useRef(0);
  const alive = useRef(true);
  const [fps, setFps] = useState(0);
  const [loading, setLoading] = useState(true);
  const [ready, setReady] = useState(false);
  const [dragging, setDragging] = useState(false);
  const [modelName, setModelName] = useState("No model loaded");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let mounted = true;
    alive.current = true;
    const request = ++loadRequest.current;
    const handleRendererError = (event: Event) => {
      const detail = (event as CustomEvent<unknown>).detail;
      if (mounted) {
        setError(typeof detail === "string" ? detail : "The renderer reported an error");
      }
    };
    document.addEventListener("renderer-error", handleRendererError);

    void (async () => {
      try {
        setLoading(true);
        setError(null);
        await initRenderer("viewer");
        if (!mounted || request !== loadRequest.current) {
          if (!mounted && request === loadRequest.current) {
            disposeRenderer();
          }
          return;
        }
        loadModelBytes(createDemoModel());
        setModelName("Built-in cube");
        setReady(true);
      } catch (caught: unknown) {
        if (mounted) {
          setError(errorMessage(caught));
        }
      } finally {
        if (mounted && request === loadRequest.current) {
          setLoading(false);
        }
      }
    })();

    return () => {
      mounted = false;
      alive.current = false;
      document.removeEventListener("renderer-error", handleRendererError);
      disposeRenderer();
    };
  }, []);

  useEffect(() => {
    const update = () => setFps(Math.round(rendererFPS()));
    const interval = window.setInterval(update, 500);
    return () => window.clearInterval(interval);
  }, []);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) {
      return;
    }
    const resize = () => {
      const bounds = canvas.getBoundingClientRect();
      const pixelRatio = Math.min(window.devicePixelRatio || 1, 2);
      const width = Math.max(1, Math.round(bounds.width * pixelRatio));
      const height = Math.max(1, Math.round(bounds.height * pixelRatio));
      if (canvas.width !== width || canvas.height !== height) {
        canvas.width = width;
        canvas.height = height;
      }
    };
    const observer = new ResizeObserver(resize);
    observer.observe(canvas);
    resize();
    return () => observer.disconnect();
  }, []);

  function handlePointerDown(event: PointerEvent<HTMLCanvasElement>) {
    event.currentTarget.setPointerCapture(event.pointerId);
    pointer.current = { id: event.pointerId, x: event.clientX, y: event.clientY };
    setDragging(true);
  }

  function handlePointerMove(event: PointerEvent<HTMLCanvasElement>) {
    const previous = pointer.current;
    if (!previous || previous.id !== event.pointerId) {
      return;
    }
    rotateRenderer(event.clientX - previous.x, event.clientY - previous.y);
    pointer.current = { id: event.pointerId, x: event.clientX, y: event.clientY };
  }

  function handlePointerEnd(event: PointerEvent<HTMLCanvasElement>) {
    if (pointer.current?.id === event.pointerId) {
      pointer.current = null;
      setDragging(false);
    }
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
  }

  async function handleFileChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.currentTarget.files?.[0];
    event.currentTarget.value = "";
    if (!file) {
      return;
    }
    const request = ++loadRequest.current;
    try {
      setLoading(true);
      setError(null);
      const data = await readModelFile(file);
      if (!alive.current || request !== loadRequest.current) {
        return;
      }
      loadModelBytes(data);
      setModelName(file.name);
    } catch (caught: unknown) {
      if (alive.current && request === loadRequest.current) {
        setError(errorMessage(caught));
      }
    } finally {
      if (alive.current && request === loadRequest.current) {
        setLoading(false);
      }
    }
  }

  return (
    <Box sx={{ minHeight: "100vh", background: "linear-gradient(180deg, #101418 0%, #0b0d10 100%)" }}>
      <AppBar position="static" elevation={0} sx={{ background: "#151a20", borderBottom: "1px solid rgba(255,255,255,0.08)" }}>
        <Toolbar>
          <ViewInArIcon sx={{ mr: 1.5 }} />
          <Typography variant="h6" sx={{ flexGrow: 1, fontWeight: 600 }}>
            Go WASM Model Viewer
          </Typography>
          <Stack direction="row" spacing={1}>
            <Chip label={ready ? "Ready" : "Starting"} color={ready ? "success" : "default"} variant="outlined" />
            <Chip
              icon={<SpeedIcon />}
              label={`${fps} FPS`}
              color={fps >= 50 ? "success" : fps >= 30 ? "warning" : "error"}
              variant="outlined"
            />
          </Stack>
        </Toolbar>
      </AppBar>

      {loading && <LinearProgress aria-label="Loading model" />}

      <Container maxWidth="xl" sx={{ py: 3 }}>
        {error && <Alert severity="error" onClose={() => setError(null)} sx={{ mb: 3 }}>{error}</Alert>}
        <Stack direction={{ xs: "column", lg: "row" }} spacing={3}>
          <Card sx={panelStyle}>
            <CardContent>
              <Typography variant="h6" gutterBottom>Scene Controls</Typography>
              <Typography variant="body2" sx={{ opacity: 0.7, mb: 2 }}>
                Upload and inspect GLB 2.0 models rendered through Go and WebAssembly.
              </Typography>
              <Divider sx={{ mb: 2, borderColor: "rgba(255,255,255,0.08)" }} />
              <Stack spacing={2}>
                <Button
                  variant="contained"
                  component="label"
                  startIcon={<CloudUploadIcon />}
                  size="large"
                  disabled={!ready || loading}
                >
                  Upload GLB
                  <input hidden type="file" accept=".glb,model/gltf-binary" onChange={handleFileChange} />
                </Button>
                <Box aria-live="polite">
                  <Typography variant="caption" sx={{ opacity: 0.6 }}>CURRENT MODEL</Typography>
                  <Typography variant="body1" sx={{ mt: 0.5, wordBreak: "break-word" }}>{modelName}</Typography>
                </Box>
                <Box>
                  <Typography variant="caption" sx={{ opacity: 0.6 }}>CONTROLS</Typography>
                  <Typography variant="body2" sx={{ mt: 0.5 }}>
                    Drag over the viewport to rotate the centered model.
                  </Typography>
                </Box>
              </Stack>
            </CardContent>
          </Card>

          <Card sx={{ flex: 1, background: "#0f1318", borderRadius: 3, overflow: "hidden", border: "1px solid rgba(255,255,255,0.08)" }}>
            <Box sx={{ display: "flex", alignItems: "center", justifyContent: "space-between", px: 2, py: 1.5, borderBottom: "1px solid rgba(255,255,255,0.08)", background: "#161b21" }}>
              <Typography variant="subtitle1" sx={{ color: "white", fontWeight: 600 }}>Viewport</Typography>
              <Chip size="small" label="WebGL2" color="primary" />
            </Box>
            <Box sx={{ display: "flex", justifyContent: "center", alignItems: "center", p: 2, background: "radial-gradient(circle at center, #1a2027 0%, #0f1318 100%)" }}>
              <canvas
                ref={canvasRef}
                id="viewer"
                width={1000}
                height={700}
                role="img"
                aria-label="Interactive 3D model viewport"
                data-testid="viewer"
                onPointerDown={handlePointerDown}
                onPointerMove={handlePointerMove}
                onPointerUp={handlePointerEnd}
                onPointerCancel={handlePointerEnd}
                style={{
                  width: "100%",
                  maxWidth: 1000,
                  aspectRatio: "10 / 7",
                  borderRadius: 12,
                  border: "1px solid rgba(255,255,255,0.08)",
                  background: "#20252b",
                  cursor: dragging ? "grabbing" : "grab",
                  touchAction: "none",
                }}
              />
            </Box>
          </Card>
        </Stack>
      </Container>
    </Box>
  );
}

const panelStyle = {
  width: { xs: "100%", lg: 320 },
  background: "#171c22",
  color: "white",
  borderRadius: 3,
  border: "1px solid rgba(255,255,255,0.08)",
};

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}
