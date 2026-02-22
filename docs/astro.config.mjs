import { defineConfig } from "astro/config";
import starlight from "@astrojs/starlight";

export default defineConfig({
  site: "https://docs.mycli.sh",
  integrations: [
    starlight({
      title: "mycli",
      logo: {
        src: "./public/logo.svg",
      },
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
            { label: "Shelves & Libraries", slug: "guides/shelves-and-libraries" },
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
              label: "Shelf Commands",
              autogenerate: { directory: "reference/shelf" },
            },
            { label: "Spec Format", slug: "reference/spec-format" },
            { label: "Shelf Manifest", slug: "reference/shelf-manifest" },
            { label: "API Endpoints", slug: "reference/api-endpoints" },
            { label: "Environment Variables", slug: "reference/environment-variables" },
          ],
        },
      ],
    }),
  ],
});
