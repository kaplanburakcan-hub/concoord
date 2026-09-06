import { fileURLToPath, URL } from "node:url";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Geliştirmede /api ve sağlık uçları backend'e proxy'lenir.
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      // frappe-gantt'ın package.json "exports" alanı ./dist/frappe-gantt.css
      // için bir alt yol tanımlamıyor; Node/Vite'ın exports zorlaması bu
      // yüzden derin import'u reddediyor — dosya gerçekten var, doğrudan
      // gerçek yola yönlendiriyoruz.
      "frappe-gantt/dist/frappe-gantt.css": fileURLToPath(
        new URL("./node_modules/frappe-gantt/dist/frappe-gantt.css", import.meta.url)
      ),
    },
  },
  server: {
    port: 5173,
    proxy: {
      "/api": "http://localhost:8080",
      "/healthz": "http://localhost:8080",
      "/readyz": "http://localhost:8080",
    },
  },
});
