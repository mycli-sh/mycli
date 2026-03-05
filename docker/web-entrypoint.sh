#!/bin/sh
set -e

# Default to port 8080 if PORT is not set
export PORT="${PORT:-8080}"

# Generate nginx config from template
envsubst '${PORT}' < /etc/nginx/conf.d/default.conf.template > /etc/nginx/conf.d/default.conf

# Replace API URL placeholder in built JS files
if [ -n "$VITE_API_URL" ]; then
  find /usr/share/nginx/html/assets -name '*.js' -exec \
    sed -i "s|__API_URL_PLACEHOLDER__|${VITE_API_URL}|g" {} +
fi

# Replace APP URL placeholder in index.html (for OG meta tags)
if [ -n "$VITE_APP_URL" ]; then
  sed -i "s|__APP_URL_PLACEHOLDER__|${VITE_APP_URL}|g" /usr/share/nginx/html/index.html
fi

# Replace PostHog key placeholder in built JS files
# If VITE_POSTHOG_KEY is unset, replaces with empty string (PostHog stays disabled)
find /usr/share/nginx/html/assets -name '*.js' -exec \
  sed -i "s|__POSTHOG_KEY_PLACEHOLDER__|${VITE_POSTHOG_KEY:-}|g" {} +
