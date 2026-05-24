# syntax=docker/dockerfile:1.7

ARG GO_VERSION=1.26
ARG BUN_VERSION=1
ARG ALPINE_VERSION=3.20

FROM --platform=$BUILDPLATFORM oven/bun:${BUN_VERSION}-alpine AS frontend

WORKDIR /app/web

RUN apk add --no-cache nodejs

COPY web/package.json web/bun.lock ./
RUN bun install --frozen-lockfile

COPY web/ ./

RUN bun run build -- --mode prod

FROM --platform=$TARGETPLATFORM golang:${GO_VERSION}-alpine AS builder

WORKDIR /app

ARG BUILD_VERSION=dev
ARG GO_TAGS=
ARG GO_MODFILE=

COPY go.mod go.sum go.private.mod go.private.sum ./
RUN if [ -z "$GO_MODFILE" ]; then \
		go mod download; \
	else \
		apk add --no-cache git; \
	fi

COPY . .
COPY --from=frontend /app/web/dist ./web/dist

RUN --mount=type=secret,id=private_module_token,required=false \
	if [ -n "$GO_MODFILE" ]; then \
		token="$(cat /run/secrets/private_module_token 2>/dev/null || true)"; \
		if [ -n "$token" ]; then \
			git config --global url."https://x-access-token:${token}@github.com/".insteadOf "https://github.com/"; \
		fi; \
		go env -w GOPRIVATE=github.com/damonto/*; \
		go mod download -modfile="$GO_MODFILE"; \
		CGO_ENABLED=0 go build -tags="$GO_TAGS" -modfile="$GO_MODFILE" -trimpath -ldflags="-w -s -X main.BuildVersion=${BUILD_VERSION}" -o sigmo .; \
	else \
		CGO_ENABLED=0 go build -trimpath -ldflags="-w -s -X main.BuildVersion=${BUILD_VERSION}" -o sigmo .; \
	fi

FROM alpine:${ALPINE_VERSION} AS runner

WORKDIR /app

COPY --from=builder /app/sigmo /app/sigmo
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

RUN set -eux \
	&& apk add --no-cache ca-certificates dbus libmbim-tools modemmanager qmi-utils \
	&& mkdir -p /run/dbus \
	&& chmod +x /usr/local/bin/docker-entrypoint.sh

ENV DBUS_SYSTEM_BUS_ADDRESS=unix:path=/run/dbus/system_bus_socket

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
