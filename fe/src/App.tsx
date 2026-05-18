import { useEffect, useRef } from "react";

import { loadWasm } from "./lib/loadWasm";

export default function App() {
  const lastMouse = useRef({
    x: 0,
    y: 0,
  });

  useEffect(() => {
    async function init() {
      await loadWasm();

      window.goInitRenderer?.("viewer");
    }

    init();
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

  return (
    <canvas
      id="viewer"
      width={800}
      height={600}
      onMouseMove={handleMouseMove}
    />
  );
}
