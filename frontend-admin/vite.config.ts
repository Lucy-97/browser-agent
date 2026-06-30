import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5174,
    proxy: {
      "/admin": "http://localhost:8080",
      "/api": "http://localhost:8080",
    },
  },
});
