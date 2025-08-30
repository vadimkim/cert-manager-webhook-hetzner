# ---- Build dependencies ----
FROM golang:1.25-alpine3.22 AS build_deps
ARG TARGETARCH

RUN apk add --no-cache git=2.49.1-r0

WORKDIR /workspace
ENV GO111MODULE=on

COPY go.mod .
COPY go.sum .

RUN go mod download

# ---- Build stage ----
FROM build_deps AS build

COPY . .

RUN CGO_ENABLED=0 GOARCH=$TARGETARCH go build -o webhook -ldflags '-w -extldflags "-static"' .

# ---- Final runtime image ----
FROM alpine:3.22
LABEL maintainer="vadimkim <vadim@ant.ee>"
LABEL org.opencontainers.image.source="https://github.com/vadimkim/cert-manager-webhook-hetzner"

# Install minimal runtime
RUN apk add --no-cache ca-certificates=20241121-r2 \
    && adduser -D -u 1000 appuser
USER appuser

COPY --from=build /workspace/webhook /usr/local/bin/webhook

ENTRYPOINT ["webhook"]

# Add healthcheck (adjust endpoint/port if needed)
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1
