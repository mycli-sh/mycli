import { defineConfig } from "astro/config";
import tailwindcss from "@tailwindcss/vite";
import posthog from "./posthog.integration.mjs";

export default defineConfig({
  site: "https://mycli.sh",
  integrations: [posthog()],
  vite: {
    plugins: [tailwindcss()],
  },
});
