# syntax=docker/dockerfile:1

### Obtain kubectl
FROM curlimages/curl:7.85.0 as kubectl
RUN curl -L -o /tmp/kubectl https://dl.k8s.io/release/v1.25.2/bin/linux/amd64/kubectl && chmod +x /tmp/kubectl

### Build executable
FROM golang:1.18 as builder
WORKDIR /workspace
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build go mod download
COPY cmd ./cmd
COPY internal ./internal
ENV CGO_ENABLED="0"
ENV GOARCH="amd64"
ENV GOOS="linux"
ENV GO111MODULE="on"
ARG SKAFFOLD_GO_GCFLAGS
RUN --mount=type=cache,target=/root/.cache/go-build go build -gcflags="${SKAFFOLD_GO_GCFLAGS}" -o controller ./cmd/main.go

### Target layer
FROM gcr.io/distroless/base-debian11
WORKDIR /
COPY --from=builder /workspace/controller ./controller
COPY --from=kubectl /tmp/kubectl /usr/local/bin/kubectl
ENV GOTRACEBACK=all
ENTRYPOINT ["/controller"]
