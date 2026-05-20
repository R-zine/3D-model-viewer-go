import { useEffect, useRef, useState } from "react";

import {
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
  ViewInAr as ViewInArIcon,
  Speed as SpeedIcon,
  CloudUpload as CloudUploadIcon,
} from "@mui/icons-material";

import { initRenderer, loadModel } from "./lib/loadWasm";

export default function App() {
  const lastMouse = useRef({
    x: 0,
    y: 0,
  });

  const frameCounter = useRef(0);

  const [fps, setFps] = useState(0);

  const [loading, setLoading] = useState(true);

  const [modelName, setModelName] = useState("No model loaded");

  useEffect(() => {
    async function init() {
      try {
        await initRenderer("viewer");
      } finally {
        setLoading(false);
      }
    }

    init();
  }, []);

  // FPS COUNTER
  useEffect(() => {
    let lastTime = performance.now();

    let animationFrame = 0;

    function updateFPS() {
      frameCounter.current++;

      const now = performance.now();

      const elapsed = now - lastTime;

      if (elapsed >= 1000) {
        const currentFPS = Math.round((frameCounter.current * 1000) / elapsed);

        setFps(currentFPS);

        frameCounter.current = 0;

        lastTime = now;
      }

      animationFrame = requestAnimationFrame(updateFPS);
    }

    animationFrame = requestAnimationFrame(updateFPS);

    return () => {
      cancelAnimationFrame(animationFrame);
    };
  }, []);

  function handleMouseMove(e: React.MouseEvent) {
    const deltaX = e.clientX - lastMouse.current.x;

    const deltaY = e.clientY - lastMouse.current.y;

    lastMouse.current = {
      x: e.clientX,
      y: e.clientY,
    };

    window.goHandleMouseMove?.(deltaX, deltaY);
  }

  async function handleFileChange(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];

    if (!file) {
      return;
    }

    try {
      setLoading(true);

      setModelName(file.name);

      await loadModel(file);
    } catch (err) {
      console.error("failed to load model", err);
    } finally {
      setLoading(false);
    }
  }

  return (
    <Box
      sx={{
        minWidth: "100vw",
        minHeight: "100vh",
        background: "linear-gradient(180deg, #101418 0%, #0b0d10 100%)",
      }}
    >
      <AppBar
        position="static"
        elevation={0}
        sx={{
          background: "#151a20",
          borderBottom: "1px solid rgba(255,255,255,0.08)",
        }}
      >
        <Toolbar>
          <ViewInArIcon sx={{ mr: 1.5 }} />

          <Typography
            variant="h6"
            sx={{
              flexGrow: 1,
              fontWeight: 600,
            }}
          >
            Go WASM Model Viewer
          </Typography>

          <Chip
            icon={<SpeedIcon />}
            label={`${fps} FPS`}
            color={fps >= 50 ? "success" : fps >= 30 ? "warning" : "error"}
            variant="outlined"
          />
        </Toolbar>
      </AppBar>

      {loading && <LinearProgress />}

      <Container maxWidth="xl" sx={{ py: 3 }}>
        <Stack
          direction={{
            xs: "column",
            lg: "row",
          }}
          spacing={3}
        >
          <Card
            sx={{
              width: {
                xs: "100%",
                lg: 320,
              },
              background: "#171c22",
              color: "white",
              borderRadius: 3,
              border: "1px solid rgba(255,255,255,0.08)",
            }}
          >
            <CardContent>
              <Typography variant="h6" gutterBottom>
                Scene Controls
              </Typography>

              <Typography
                variant="body2"
                sx={{
                  opacity: 0.7,
                  mb: 2,
                }}
              >
                Upload and inspect GLB models rendered through Go + WebAssembly.
              </Typography>

              <Divider
                sx={{
                  mb: 2,
                  borderColor: "rgba(255,255,255,0.08)",
                }}
              />

              <Stack spacing={2}>
                <Button
                  variant="contained"
                  component="label"
                  startIcon={<CloudUploadIcon />}
                  size="large"
                >
                  Upload GLB
                  <input
                    hidden
                    type="file"
                    accept=".glb"
                    onChange={handleFileChange}
                  />
                </Button>

                <Box>
                  <Typography
                    variant="caption"
                    sx={{
                      opacity: 0.6,
                    }}
                  >
                    CURRENT MODEL
                  </Typography>

                  <Typography
                    variant="body1"
                    sx={{
                      mt: 0.5,
                      wordBreak: "break-word",
                    }}
                  >
                    {modelName}
                  </Typography>
                </Box>

                <Box>
                  <Typography
                    variant="caption"
                    sx={{
                      opacity: 0.6,
                    }}
                  >
                    CONTROLS
                  </Typography>

                  <Typography variant="body2" sx={{ mt: 0.5 }}>
                    Move the mouse over the canvas to rotate the model.
                  </Typography>
                </Box>
              </Stack>
            </CardContent>
          </Card>

          <Card
            sx={{
              flex: 1,
              background: "#0f1318",
              borderRadius: 3,
              overflow: "hidden",
              border: "1px solid rgba(255,255,255,0.08)",
            }}
          >
            <Box
              sx={{
                display: "flex",
                alignItems: "center",
                justifyContent: "space-between",
                px: 2,
                py: 1.5,
                borderBottom: "1px solid rgba(255,255,255,0.08)",
                background: "#161b21",
              }}
            >
              <Typography
                variant="subtitle1"
                sx={{
                  color: "white",
                  fontWeight: 600,
                }}
              >
                Viewport
              </Typography>

              <Chip size="small" label="WebGL2" color="primary" />
            </Box>

            <Box
              sx={{
                display: "flex",
                justifyContent: "center",
                alignItems: "center",
                p: 2,
                background:
                  "radial-gradient(circle at center, #1a2027 0%, #0f1318 100%)",
              }}
            >
              <canvas
                id="viewer"
                width={1000}
                height={700}
                onMouseMove={handleMouseMove}
                style={{
                  width: "100%",
                  maxWidth: 1000,
                  height: "auto",
                  borderRadius: 12,
                  border: "1px solid rgba(255,255,255,0.08)",
                  background: "#20252b",
                  cursor: "grab",
                }}
              />
            </Box>
          </Card>
        </Stack>
      </Container>
    </Box>
  );
}
