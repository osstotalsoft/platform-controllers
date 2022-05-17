FROM alpine:latest as alpine
RUN apk add -U --no-cache ca-certificates curl
RUN curl -fsSL https://get.pulumi.com | sh
#RUN /root/.pulumi/bin/pulumi plugin install resource azure-native 1.60.0

FROM gcr.io/distroless/static:nonroot
ARG PKG_FILES
WORKDIR /
COPY --from=alpine /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=alpine /root/.pulumi/ /nonroot/.pulumi/
COPY /$PKG_FILES /
ENV PATH="/nonroot/.pulumi/bin:${PATH}"
