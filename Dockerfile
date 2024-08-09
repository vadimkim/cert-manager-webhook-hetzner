FROM golang:1.22-alpine3.20 AS build_deps
ARG TARGETARCH

RUN apk add --no-cache git

WORKDIR /workspace
ENV GO111MODULE=on

COPY go.mod .
COPY go.sum .

RUN go mod download

FROM build_deps AS build

COPY . .

RUN CGO_ENABLED=0 GOARCH=$TARGETARCH go build -o webhook -ldflags '-w -extldflags "-static"' .

FROM alpine:3.20
LABEL maintainer="Josip Nedic <josip.nedic@hotmail.de>"
LABEL org.opencontainers.image.source="https://github.com/josobrate/cert-manager-webhook-hosting-de"

RUN apk add --no-cache ca-certificates

COPY --from=build /workspace/webhook /usr/local/bin/webhook

ENTRYPOINT ["webhook"]
