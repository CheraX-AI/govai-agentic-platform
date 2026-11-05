# GovAI web app. One Dockerfile, two targets (see the root docker-compose.yml):
#
#   dev  - Vite dev server with live reload; src/, public/ and index.html are mounted
#   prod - production build (npm run build) served by nginx over HTTPS
#
# Both serve https://localhost:5173, the origin auth-service accepts (WEB_ORIGIN,
# CORS_ALLOWED_ORIGINS). The local TLS certificate is mounted at runtime and is
# never copied into an image.

# ---------------------------------------------------------------------------
# deps: locked dependencies, installed as the non-root "node" user
# ---------------------------------------------------------------------------
FROM node:22-alpine AS deps
WORKDIR /app/web
RUN chown node:node /app/web
USER node
COPY --chown=node:node package.json package-lock.json ./
RUN npm ci

# ---------------------------------------------------------------------------
# dev: Vite dev server
# ---------------------------------------------------------------------------
FROM deps AS dev
COPY --chown=node:node . .
EXPOSE 5173
# --host overrides `host: 'localhost'` in vite.config.ts so the published port works
CMD ["npx", "vite", "--host", "0.0.0.0"]

# ---------------------------------------------------------------------------
# build: type check and production bundle (dist/)
# ---------------------------------------------------------------------------
FROM deps AS build
COPY --chown=node:node . .
# Auth API address for the browser; Vite writes it into the bundle at build time
ARG VITE_AUTH_API_URL=https://localhost:5001
ENV VITE_AUTH_API_URL=${VITE_AUTH_API_URL}
RUN npm run build

# ---------------------------------------------------------------------------
# prod: nginx (non-root image) serving dist/ over HTTPS on 8443
# ---------------------------------------------------------------------------
FROM nginxinc/nginx-unprivileged:1.27-alpine AS prod
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/web/dist /usr/share/nginx/html
EXPOSE 8443
