import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";

// The app is served by the PC (Go, embedded) from public/app.
export default defineConfig({
  plugins: [svelte()],
  base: "/",
  build: { outDir: "../public/app", emptyOutDir: true, target: "es2020", cssTarget: ["safari15", "chrome100"], assetsInlineLimit: 0, sourcemap: false },
  server: { proxy: { "/ws": { target: "ws://localhost:47615", ws: true }, "/api": "http://localhost:47615", "/gws": { target: "ws://localhost:47616", ws: true } } },
});
