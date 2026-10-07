import { defineConfig, mergeConfig } from "vitest/config";
import viteConfig from "./vite.config";

// Birim + bileşen testleri (jsdom). `npm test` ile çalışır; CI'da zorunludur.
export default mergeConfig(
  viteConfig,
  defineConfig({
    test: {
      environment: "jsdom",
      setupFiles: ["./src/test/setup.ts"],
      include: ["src/**/*.test.{ts,tsx}"],
      restoreMocks: true,
    },
  })
);
