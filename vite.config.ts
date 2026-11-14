import { fileURLToPath } from "node:url";
import { defineConfig, type UserConfig } from "vite";

// `vite --mode preview` is the only server that replaces mygo-runtime.
// `bun run dev:web` stays on the real runtime.
export default defineConfig(({ mode }): UserConfig => {
  if (mode !== "preview") return {};
  return {
    plugins: [
      {
        name: "preview-no-external-fonts",
        transform(code, id) {
          if (id.endsWith("/src/ui/bundle.css")) {
            return code.replace(/^@import\s+url\([^)]*fonts\.googleapis\.com[^)]*\);?\s*/m, "");
          }
        },
        transformIndexHtml(html) {
          return html.replace("</head>", '<link rel="icon" type="image/png" href="../assets/icon.png"></head>');
        },
      },
    ],
    resolve: {
      alias: {
        "mygo-runtime": fileURLToPath(new URL("./src/preview/runtime.ts", import.meta.url)),
      },
    },
    server: { host: "127.0.0.1", port: 4173, strictPort: true },
  };
});
