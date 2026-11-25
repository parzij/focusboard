import { defineConfig } from "vite";
import tailwind from "@tailwindcss/vite";

export default defineConfig({
  plugins: [tailwind()],
  server: {
    host: "127.0.0.1",
    port: 5173,
    strictPort: true,
    proxy: { "/api/v1": { target: "http://127.0.0.1:8080" } },
  },
});
