FROM golang:1.27-alpine3.23 AS build

COPY . /root/dvpnd/

RUN --mount=target=/go/pkg/mod,type=cache \
    --mount=target=/root/.cache/go-build,type=cache \
    apk add autoconf automake bash file g++ gcc git libtool linux-headers make musl-dev unbound-dev && \
    cd /root/dvpnd/ && make --jobs=$(nproc) install && \
    git clone --branch=v2.0.0 --depth=1 https://github.com/handshake-org/hnsd.git /root/hnsd && \
    git -C /root/hnsd rev-parse HEAD | grep -q ^a5c7c287e848 && \
    cd /root/hnsd/ && bash autogen.sh && sh configure && make --jobs=$(nproc)

FROM alpine:3.23

COPY --from=build /go/bin/dvpnd /usr/local/bin/process
COPY --from=build /root/hnsd/hnsd /usr/local/bin/hnsd

RUN apk add --no-cache iptables unbound-libs v2ray wireguard-tools && \
    rm -rf /etc/v2ray/ /usr/share/v2ray/

CMD ["process"]
