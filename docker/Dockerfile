FROM alpine:latest as builder
RUN apk add -U --no-cache ca-certificates curl
RUN curl -fsSL https://get.pulumi.com | sh
#RUN /root/.pulumi/bin/pulumi plugin install resource azure-native 1.60.0

FROM alpine:latest
ARG PKG_FILES
WORKDIR /

RUN apk add -U --no-cache powershell bash

RUN addgroup -g 1300 -S nonroot \
    && adduser -u 1300 -S nonroot -G nonroot

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder --chown=nonroot /root/.pulumi/ /home/nonroot/.pulumi/
COPY --chown=nonroot /$PKG_FILES /
ENV PATH="/home/nonroot/.pulumi/bin:${PATH}"
USER nonroot
