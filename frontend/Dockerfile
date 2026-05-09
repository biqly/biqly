# Builder: install deps and produce a static dist/ via Vite.
FROM node:20-alpine AS builder

WORKDIR /src

COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

# Runtime: nginx serving the built bundle. SPA fallback to index.html.
FROM nginx:1.27-alpine

RUN rm -rf /usr/share/nginx/html/* \
    && rm /etc/nginx/conf.d/default.conf

COPY --from=builder /src/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 3333

# nginx official image already drops to the unprivileged user via its
# entrypoint scripts; we keep the default CMD.
