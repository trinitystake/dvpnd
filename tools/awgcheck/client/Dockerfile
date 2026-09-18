# An AmneziaWG client: amneziawg-go and the tools at the given tags, the way
# a client app bundles them.
FROM golang:alpine AS build
ARG GO_TAG
ARG TOOLS_TAG
RUN apk add --no-cache bash gcc git linux-headers make musl-dev
RUN git clone --quiet --branch=${GO_TAG} --depth=1 https://github.com/amnezia-vpn/amneziawg-go.git /src/go && \
    cd /src/go && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/amneziawg-go . && \
    git clone --quiet --branch=${TOOLS_TAG} --depth=1 https://github.com/amnezia-vpn/amneziawg-tools.git /src/tools && \
    make -C /src/tools/src --jobs=$(nproc) && \
    make -C /src/tools/src DESTDIR=/out/tools PREFIX=/usr WITH_WGQUICK=yes WITH_BASHCOMPLETION=no WITH_SYSTEMDUNITS=no install

FROM alpine:3.24
RUN apk add --no-cache bash curl iproute2 iptables
COPY --from=build /out/amneziawg-go /usr/bin/amneziawg-go
COPY --from=build /out/tools/usr/bin/awg /out/tools/usr/bin/awg-quick /usr/bin/
COPY run-client.sh /usr/local/bin/run-client
RUN chmod 0755 /usr/local/bin/run-client
ENTRYPOINT ["/usr/local/bin/run-client"]
