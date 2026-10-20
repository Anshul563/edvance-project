# MediaMTX with a shell and curl so runOnOnline/runOnOffline hooks can
# call the live-streaming-service callbacks. The upstream image is
# scratch-based and cannot run hook commands.
#
# The release binary is amd64; on Apple Silicon run docker with
# `platform: linux/amd64` or change MEDIAMTX_ARCH to arm64v8.
FROM alpine:3.20

ARG MEDIAMTX_VERSION=1.21.2
ARG MEDIAMTX_ARCH=amd64

RUN apk add --no-cache ca-certificates curl wget \
    && curl -fsSL -o /tmp/mediamtx.tar.gz \
        "https://github.com/bluenviron/mediamtx/releases/download/v${MEDIAMTX_VERSION}/mediamtx_v${MEDIAMTX_VERSION}_linux_${MEDIAMTX_ARCH}.tar.gz" \
    && tar -xzf /tmp/mediamtx.tar.gz -C /usr/local/bin mediamtx \
    && chmod +x /usr/local/bin/mediamtx \
    && rm /tmp/mediamtx.tar.gz

EXPOSE 1935 8888 9997

ENTRYPOINT ["/usr/local/bin/mediamtx"]
CMD ["/etc/mediamtx/mediamtx.yml"]
