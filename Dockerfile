FROM node:24-alpine AS web
WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26.9-alpine AS backend
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o /out/faultharbor ./cmd/faultharbor

RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o /out/faultharbor-collector ./cmd/faultharbor-collector

FROM alpine:3.24
RUN apk add --no-cache ca-certificates && mkdir -p /run/faultharbor && chown 0:65532 /run/faultharbor && chmod 0750 /run/faultharbor
WORKDIR /app
COPY --from=backend /out/ /usr/local/bin/
COPY --from=web /src/web/dist ./web/dist
ENV FAULTHARBOR_ADDR=0.0.0.0:8080
USER 65532:65532
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/health/ready || exit 1
ENTRYPOINT ["faultharbor"]
