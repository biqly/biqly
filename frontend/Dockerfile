# Builder: install deps and produce a static dist/ via Vite.
FROM node:22-alpine AS builder

WORKDIR /src

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

RUN npm run build

# Runtime: nginx serving the built bundle. SPA fallback to index.html.
# nginx-unprivileged runs entirely as a non-root user (uid 101) and writes its
# pid and temp files under /tmp, so no root is required at runtime.
FROM ghcr.io/nginx/nginx-unprivileged:1.31.1-alpine

# The base image already ships a default.conf (listen 8080); our COPY overwrites
# it. dist/ is copied over the image's default html (index.html is replaced).
COPY --from=builder /src/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

# nginx-unprivileged listens on 8080 (see nginx.conf); the old 3333 was the Vite dev port.
EXPOSE 8080
