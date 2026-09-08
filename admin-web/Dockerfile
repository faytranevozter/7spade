FROM node:24-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
ARG VITE_ADMIN_API_URL
ENV VITE_ADMIN_API_URL=${VITE_ADMIN_API_URL}
RUN npm run build

FROM nginx:alpine
COPY --from=builder /app/dist /usr/share/nginx/html
# The official nginx entrypoint renders templates with envsubst. Restrict it to
# this variable so nginx runtime variables in future config changes stay intact.
ENV NGINX_ENVSUBST_FILTER=^SKIN_ASSETS_ORIGIN$
COPY nginx.conf /etc/nginx/templates/default.conf.template
EXPOSE 80
