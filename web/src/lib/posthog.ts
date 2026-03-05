import type PostHogJs from "posthog-js";

const key = import.meta.env.VITE_POSTHOG_KEY;
const host = import.meta.env.VITE_POSTHOG_HOST || "https://us.i.posthog.com";

let posthog: PostHogJs | undefined;

if (key) {
  const mod = await import("posthog-js");
  posthog = mod.default;
  posthog.init(key, {
    api_host: host,
    person_profiles: "identified_only",
    capture_pageview: false,
  });
}

export { posthog };
