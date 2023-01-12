FROM docker.io/alpine:3
ARG WEBHOOK_ARTIFACT_PATH="./webhook"

RUN \
  apk upgrade && \
  apk add ca-certificates && \
  rm -rf /var/cache/apk/*

COPY "${WEBHOOK_ARTIFACT_PATH}" /usr/local/bin/webhook

ENTRYPOINT ["/usr/local/bin/webhook"]
