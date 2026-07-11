import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Geliştirmede /api ve sağlık uçları backend'e proxy'lenir.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    proxy: {
      "/api": "http://localhost:8080",
      "/healthz": "http://localhost:8080",
      "/readyz": "http://localhost:8080",
    },
  },
});
