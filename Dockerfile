ARG BUILD_ARCH=amd64
FROM golang:1.20.14 AS builder

ARG GOPROXY=https://goproxy.cn
ARG BUILD_ARCH

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GO111MODULE=on \
    GOSUMDB=off \
    GOPROXY=${GOPROXY}

WORKDIR /go/pvc-operator

COPY . /go/pvc-operator

RUN set -x \
    && go get ./cmd/cronpva/ \
    && GOOS=linux GOARCH=${BUILD_ARCH} \
    go build -a -ldflags '-s' -o "_output/bin/${BUILD_ARCH}/cronpva-operator" ./cmd/cronpva

FROM --platform=$BUILD_ARCH alpine:3.21.3
ARG BUILD_ARCH
USER root
ENV START_FLAG=""

COPY --from=builder /go/pvc-operator/_output/bin/${BUILD_ARCH}/cronpva-operator /cronpva-operator

RUN set -x \
    && chmod +x /cronpva-operator

WORKDIR /
ENTRYPOINT ["/cronpva-operator"]