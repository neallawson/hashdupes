import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import path from "node:path";

export default defineConfig({
  plugins: [svelte()],
  resolve: {
    alias: {
      $lib: path.resolve(__dirname, "src/lib"),
    },
  },
  server: {
    port: 5173,
    strictPort: true,
  },
  build: {
    // Wails embeds frontend/dist; keep output deterministic.
    outDir: "dist",
    emptyOutDir: true,
  },
});
