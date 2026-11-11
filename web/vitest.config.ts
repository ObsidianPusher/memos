import { fileURLToPath } from "node:url";
import { defineConfig } from "vitest/config";

export default defineConfig({
  resolve: {
    alias: [
      { find: "@", replacement: fileURLToPath(new URL("./src", import.meta.url)) },
      { find: /^react$/, replacement: fileURLToPath(new URL("./node_modules/react/index.js", import.meta.url)) },
      { find: /^react-dom$/, replacement: fileURLToPath(new URL("./node_modules/react-dom/index.js", import.meta.url)) },
      { find: /^react-dom\//, replacement: fileURLToPath(new URL("./node_modules/react-dom/", import.meta.url)) },
    ],
  },
  test: { setupFiles: ["tests/setup.ts"], environment: "jsdom", include: ["tests/**/*.test.tsx"] },
});
