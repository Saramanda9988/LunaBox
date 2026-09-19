import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  root: "admin",
  base: "/admin/",
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/v1": "http://localhost:8787",
    },
  },
  build: {
    outDir: "../dist/admin",
    emptyOutDir: true,
    target: "es2022",
    // The optional Ant Design Plots chunk is loaded only when chart data exists.
    chunkSizeWarningLimit: 1600,
  },
});
