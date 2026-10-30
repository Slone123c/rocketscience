import { fileURLToPath } from "node:url";
import { defineConfig, type UserConfig } from "vite";

// `vite --mode preview` is the only server that replaces mygo-runtime.
// `bun run dev:web` stays on the real runtime.
export default defineConfig(({ mode }): UserConfig => {
  if (mode !== "preview") return {};
  return {
    resolve: {
      alias: {
        "mygo-runtime": fileURLToPath(new URL("./src/preview/runtime.ts", import.meta.url)),
      },
    },
    server: { host: "127.0.0.1", port: 4173, strictPort: true },
  };
});
