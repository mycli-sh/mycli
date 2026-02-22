#!/bin/sh
set -e

if [ -n "$VITE_API_URL" ]; then
  find /usr/share/nginx/html/assets -name '*.js' -exec \
    sed -i "s|__API_URL_PLACEHOLDER__|${VITE_API_URL}|g" {} +
fi
