export default function posthog() {
  return {
    name: "posthog",
    hooks: {
      "astro:config:setup": ({ injectScript }) => {
        if (!process.env.PUBLIC_POSTHOG_KEY) return;
        injectScript(
          "page",
          `
          import posthog from "posthog-js";
          const key = import.meta.env.PUBLIC_POSTHOG_KEY;
          const host = import.meta.env.PUBLIC_POSTHOG_HOST || "https://us.i.posthog.com";
          if (key) {
            posthog.init(key, {
              api_host: host,
              person_profiles: "identified_only",
              capture_pageview: true,
            });
          }
          `
        );
      },
    },
  };
}
