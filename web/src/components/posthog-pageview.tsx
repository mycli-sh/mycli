import { useEffect } from "react";
import { useLocation } from "react-router";
import { posthog } from "../lib/posthog";

export function PostHogPageview() {
  const location = useLocation();

  useEffect(() => {
    posthog?.capture("$pageview", {
      $current_url: window.location.href,
    });
  }, [location]);

  return null;
}
