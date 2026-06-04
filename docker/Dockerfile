FROM alpine:latest as builder
RUN apk add -U --no-cache ca-certificates curl

FROM alpine:latest
ARG PKG_FILES
WORKDIR /

RUN apk add -U --no-cache ca-certificates curl powershell bash jq

RUN addgroup -g 1300 -S nonroot \
    && adduser -u 1300 -S nonroot -G nonroot

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --chown=nonroot /$PKG_FILES /
USER nonroot
