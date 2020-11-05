# Building stage
FROM golang:1.13-alpine3.12 as builder

WORKDIR /go/src/git.code.oa.com/tcnp/

# Source code, building tools and dependences
COPY . /go/src/github.com/yaoice/meliodas

ENV CGO_ENABLED 0
ENV GOOS linux
# when using go mod turn it on
ENV GO111MODULE=off

ENV TIMEZONE "Asia/Shanghai"

RUN sh -c "mkdir -p /opt/cni/bin && tar -zxpf meliodas/official-plugins/cni-plugins-amd64-v0.7.5.tgz -C /opt/cni/bin/ && cp -f meliodas/output/linux/amd64/* /opt/cni/bin/"

# Production stage
FROM alpine:3.12

# copy the go binaries from the building stage
RUN mkdir -p /opt/cni/bin
COPY --from=builder /opt/cni/bin /opt/cni/bin

CMD ["sh", "-c", "while true; do sleep 30; done"]
