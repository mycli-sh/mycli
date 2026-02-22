FROM oven/bun:1 AS builder

WORKDIR /app

COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile

COPY web/ .

ENV VITE_API_URL=__API_URL_PLACEHOLDER__
RUN bun run build

FROM nginx:alpine

COPY docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY docker/web-entrypoint.sh /docker-entrypoint.d/40-env-replace.sh
RUN chmod +x /docker-entrypoint.d/40-env-replace.sh

COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 80
