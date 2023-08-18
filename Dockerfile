# Build the manager binary
FROM jfrog.fkinternal.com/alm/golang:1.19-buster as builder
ARG TARGETOS
ARG TARGETARCH

ENV GO111MODULE=on \
#    GOPROXY=http://10.47.104.84 \
    CGO_ENABLED=0 \
    GOOS=${TARGETOS:-linux} \
    GOARCH=${TARGETARCH:-amd64} \
    GOPROXY="http://10.24.14.195/artifactory/api/go/go_virtual" \
    GOPRIVATE="github.fkinternal.com/*" \
    GOCACHE=$FLOW_CACHE_GOLANG

WORKDIR /workspace

#RUN mkdir -p /tmp/mod-cache
#COPY go.mod /tmp/mod-cache
#COPY go.sum /tmp/mod-cache
#RUN cd /tmp/mod-cache && \
#    go mod download -x && \
#    cd - && \
#    rm -rf /tmp/mod-cache
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

#COPY vendor/ vendor/
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/main.go cmd/main.go
COPY api/ api/
COPY pkg/controller/ internal/controller/
COPY pkg/ pkg/


# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN go build -a -o manager cmd/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM golang:1.19-buster
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]