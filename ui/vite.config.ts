import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    // Build directly into the Go backend's static directory
    outDir: "../internal/webui/static",
    // Required to empty the directory since it's outside the Vite root
    emptyOutDir: true,
  },
  server: {
    // Proxy API requests to the Go backend during development
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
});
