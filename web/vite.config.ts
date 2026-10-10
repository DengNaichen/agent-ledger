import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  build: {
    outDir: "../internal/web/dist/app",
    emptyOutDir: true,
  },
  server: {
    proxy: {
      "/api": process.env.INFTASK_API_URL ?? "http://127.0.0.1:8787",
    },
  },
});

