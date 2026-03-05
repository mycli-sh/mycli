import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";
import posthog from "./posthog.integration.mjs";

export default defineConfig({
  site: "https://docs.mycli.sh",
  integrations: [
    posthog(),
    starlight({
      title: "mycli",
      logo: {
        dark: "./public/logo.svg",
        light: "./public/logo-light.svg",
        replacesTitle: true,
      },
      head: [
        {
          tag: "script",
          attrs: { is: "inline" },
          content:
            'if (!("starlight-theme" in localStorage)) { localStorage.setItem("starlight-theme", "dark"); }',
        },
        {
          tag: "link",
          attrs: {
            rel: "icon",
            href: "/favicon.svg",
            type: "image/svg+xml",
          },
        },
        {
          tag: "link",
          attrs: {
            rel: "preconnect",
            href: "https://fonts.googleapis.com",
          },
        },
        {
          tag: "link",
          attrs: {
            rel: "preconnect",
            href: "https://fonts.gstatic.com",
            crossorigin: true,
          },
        },
        {
          tag: "link",
          attrs: {
            rel: "stylesheet",
            href: "https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600;700&family=JetBrains+Mono:wght@400;500;700&display=swap",
          },
        },
      ],
      social: [
        { icon: "github", label: "GitHub", href: "https://github.com/mycli-sh/mycli" },
      ],
      customCss: ["./src/styles/custom.css"],
      sidebar: [
        {
          label: "Getting Started",
          items: [
            { label: "Installation", slug: "getting-started/installation" },
            { label: "Quickstart", slug: "getting-started/quickstart" },
          ],
        },
        {
          label: "Guides",
          items: [
            { label: "Creating Commands", slug: "guides/creating-commands" },
            { label: "Publishing Commands", slug: "guides/publishing-commands" },
            { label: "Running Commands", slug: "guides/running-commands" },
            { label: "Template Variables", slug: "guides/template-variables" },
            { label: "Libraries", slug: "guides/libraries" },
            { label: "Authentication", slug: "guides/authentication" },
          ],
        },
        {
          label: "Reference",
          items: [
            {
              label: "CLI Commands",
              autogenerate: { directory: "reference/cli" },
            },
            {
              label: "Source Commands",
              autogenerate: { directory: "reference/source" },
            },
            {
              label: "Library Commands",
              autogenerate: { directory: "reference/library" },
            },
            { label: "Spec Format", slug: "reference/spec-format" },
            { label: "Library Manifest", slug: "reference/library-manifest" },
            { label: "API Endpoints", slug: "reference/api-endpoints" },
            { label: "Environment Variables", slug: "reference/environment-variables" },
          ],
        },
      ],
    }),
  ],
});
