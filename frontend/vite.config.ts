import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  server: {
    host: "127.0.0.1",
    allowedHosts: [".ts.net", ".ngrok-free.app", ".ngrok.io"],
    proxy: {
      "/api": "http://localhost:8080",
      "/hooks": "http://localhost:8080",
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: false,
  },
});
