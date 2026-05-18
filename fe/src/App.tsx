import { useEffect } from "react";

import { loadWasm } from "./lib/loadWasm";

export default function App() {
  useEffect(() => {
    async function init() {
      await loadWasm();

      window.goInitRenderer?.("viewer");
    }

    init();
  }, []);

  return <canvas id="viewer" width={800} height={600} />;
}
