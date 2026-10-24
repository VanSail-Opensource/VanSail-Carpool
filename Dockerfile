# syntax=docker/dockerfile:1.18

FROM node:26-alpine AS frontend-build
WORKDIR /src

COPY web/package.json web/package-lock.json ./web/
RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    cd web && npm ci

COPY api ./api
COPY web ./web
RUN mkdir -p /src/internal/webui/dist \
    && cd web \
    && npm run generate:api \
    && npm run build

FROM golang:1.27.3-alpine AS go-build
ARG TARGETOS=linux
ARG TARGETARCH
WORKDIR /src

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked go mod download

COPY . .
COPY --from=frontend-build /src/internal/webui/dist ./internal/webui/dist

RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/vansail-carpool ./cmd/server

FROM alpine:3.24 AS runtime
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

LABEL org.opencontainers.image.title="VanSail Carpool" \
      org.opencontainers.image.description="Telegram Mini App for VanSail Carpool" \
      org.opencontainers.image.source="https://github.com/VanSail-Opensource/VanSail-Carpool" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${BUILD_DATE}"

RUN apk add --no-cache ca-certificates tzdata postgresql18-client \
    && addgroup -S -g 10001 txcarpool \
    && adduser -S -D -H -u 10001 -G txcarpool txcarpool \
    && install -d -o txcarpool -g txcarpool /data

COPY --from=go-build --chown=10001:10001 /out/vansail-carpool /usr/local/bin/vansail-carpool
COPY docs/licenses /usr/share/licenses/vansail-carpool

ENV PORT=8080 \
    DATA_DIR=/data \
    TZ=UTC

USER 10001:10001
VOLUME ["/data"]
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD ["/usr/local/bin/vansail-carpool", "healthcheck", "--url", "http://127.0.0.1:8080/healthz"]

ENTRYPOINT ["/usr/local/bin/vansail-carpool"]
CMD ["serve"]
