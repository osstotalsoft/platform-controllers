FROM alpine:latest as alpine
RUN apk add -U --no-cache ca-certificates curl
RUN curl -fsSL https://get.pulumi.com | sh
#RUN /root/.pulumi/bin/pulumi plugin install resource azure-native 1.60.0

FROM gcr.io/distroless/static:debug-nonroot
ARG PKG_FILES
WORKDIR /
COPY --from=alpine /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=alpine --chown=nonroot /root/.pulumi/ /home/nonroot/.pulumi/
COPY  --chown=nonroot /$PKG_FILES /
ENV PATH="/home/nonroot/.pulumi/bin:${PATH}"
