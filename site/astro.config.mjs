import { defineConfig } from "astro/config";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  site: "https://mycli.sh",
  vite: {
    plugins: [tailwindcss()],
  },
});
