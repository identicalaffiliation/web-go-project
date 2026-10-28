# исходники фронта приходят из отдельного репозитория через именованный
# контекст "app" (см. additional_contexts в compose.yml)

FROM node:22-alpine AS build

WORKDIR /app

COPY --from=app package.json package-lock.json ./
RUN npm ci

# копируем явно, чтобы не затащить node_modules/dist с хоста
COPY --from=app index.html vite.config.ts components.json tsconfig*.json ./
COPY --from=app public ./public
COPY --from=app src ./src

RUN npm run build


FROM nginx:1.27-alpine

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html

EXPOSE 80
