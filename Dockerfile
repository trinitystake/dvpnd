FROM golang:1.27-alpine3.23 AS build

# Proxy binaries the node drives, pinned to the releases the client apps are
# tested against and checked against the sha256 the project publishes.
ARG XRAY_VERSION=v26.3.27
ARG XRAY_SHA256=23cd9af937744d97776ee35ecad4972cf4b2109d1e0fe6be9930467608f7c8ae
ARG HYSTERIA_VERSION=app/v2.10.0
ARG HYSTERIA_SHA256=04f7804159ef1d798de12a817d73aab4b9040ebe45fc62e223000c5c59e987fe
RUN apk add --no-cache unzip && \
    wget -qO /tmp/xray.zip "https://github.com/XTLS/Xray-core/releases/download/${XRAY_VERSION}/Xray-linux-64.zip" && \
    echo "${XRAY_SHA256}  /tmp/xray.zip" | sha256sum -c - && \
    unzip -q /tmp/xray.zip xray -d /tmp/xray && chmod 0755 /tmp/xray/xray && \
    wget -qO /tmp/hysteria "https://github.com/apernet/hysteria/releases/download/${HYSTERIA_VERSION}/hysteria-linux-amd64" && \
    echo "${HYSTERIA_SHA256}  /tmp/hysteria" | sha256sum -c - && chmod 0755 /tmp/hysteria

COPY . /root/dvpnd/

RUN --mount=target=/go/pkg/mod,type=cache \
    --mount=target=/root/.cache/go-build,type=cache \
    apk add autoconf automake bash file g++ gcc git libtool linux-headers make musl-dev unbound-dev && \
    cd /root/dvpnd/ && make --jobs=$(nproc) install && \
    git clone --branch=v2.0.0 --depth=1 https://github.com/handshake-org/hnsd.git /root/hnsd && \
    git -C /root/hnsd rev-parse HEAD | grep -q ^a5c7c287e848 && \
    cd /root/hnsd/ && bash autogen.sh && sh configure && make --jobs=$(nproc)

FROM alpine:3.24

COPY --from=build /go/bin/dvpnd /usr/local/bin/process
COPY --from=build /root/hnsd/hnsd /usr/local/bin/hnsd
COPY --from=build /tmp/xray/xray /usr/local/bin/xray
COPY --from=build /tmp/hysteria /usr/local/bin/hysteria

RUN apk add --no-cache iptables unbound-libs v2ray wireguard-tools && \
    rm -rf /etc/v2ray/ /usr/share/v2ray/

CMD ["process"]
