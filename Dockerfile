FROM golang:1.12.7-alpine3.10

# Add build tools
RUN apk update && \
    apk add --no-cache git

RUN go get -u github.com/golang/dep/cmd/dep
RUN go get -u github.com/onsi/ginkgo
RUN go install github.com/onsi/ginkgo/ginkgo

ENV SRC_DIR=/go/src/github.com/mattkelly/containership-test-v2-experiment/
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV KUBECONFIG=/app/kube.conf

WORKDIR $SRC_DIR

# Install deps before adding rest of source so we can cache the resulting vendor dir
COPY Gopkg.toml Gopkg.lock ./
RUN dep ensure -vendor-only

# Add the source code:
COPY . ./

# Compile test binaries to avoid compiling for each run
RUN ginkgo build -r tests
