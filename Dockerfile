# syntax=docker/dockerfile:1
FROM node:22-alpine AS frontend
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM golang:1.26-alpine AS backend
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./backend/
RUN cd backend && CGO_ENABLED=0 GOFLAGS=-mod=mod go build -trimpath -ldflags="-s -w" -o /out/videocms ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ffmpeg ca-certificates tzdata
COPY --from=backend /out/videocms /usr/local/bin/videocms
COPY --from=frontend /app/frontend/dist /srv/videocms
ENV WEB_ROOT=/srv/videocms \
    DATA_DIR=/data \
    PORT=8080
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["videocms"]
