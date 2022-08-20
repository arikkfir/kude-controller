# syntax=docker/dockerfile:1

### Build executable
FROM golang:1.19 as builder
WORKDIR /workspace

# Copy the Go manifests, download dependencies & cache them before building and copying actual source code, so when
# source code changes, downloaded dependencies stay cached and are not downloaded again (unless manifest changes too.)
# We also use the "--mount" option to mount the Go cache directory into the container, so that Go can use it.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/.cache/go-build go mod download

# Now build the actual executable
COPY cmd ./cmd
COPY internal ./internal
ENV CGO_ENABLED="0"
ENV GOARCH="amd64"
ENV GOOS="linux"
ENV GO111MODULE="on"
RUN --mount=type=cache,target=/root/.cache/go-build go build -o controller ./cmd/main.go

### Target layer
FROM gcr.io/distroless/base-debian11
WORKDIR /
COPY --from=builder /workspace/controller ./controller
ENV GOTRACEBACK=all
ENTRYPOINT ["/controller"]
