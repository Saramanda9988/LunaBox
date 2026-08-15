import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";
import UnoCSS from "unocss/vite";
import { defineConfig } from "vite";

declare const process: { env: Record<string, string | undefined> };

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), UnoCSS(), wails("./bindings")],
  optimizeDeps: {
    // The Wails plugin resolves the runtime during buildStart. Excluding this
    // ESM-only package avoids a Vite 7 optimizer race during dependency scans.
    exclude: ["@wailsio/runtime"],
  },
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
    proxy: {
      "/proxy/image": {
        target: "http://127.0.0.1:23680",
        changeOrigin: true,
      },
    },
  },
  build: {
    minify: "esbuild",
    target: "es2020",
    rollupOptions: {
      output: {
        manualChunks: {
          // 将 html2canvas 单独分包（按需加载）
          html2canvas: ["html2canvas"],
          // chart.js 相关
          chart: ["chart.js", "react-chartjs-2"],
        },
      },
    },
  },
});
