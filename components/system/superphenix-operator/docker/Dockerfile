# Build the manager binary
FROM golang:1.25 AS builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace
# Copy workspace and module definitions
COPY go.work* go.mod* go.sum* ./
COPY internal/superphenix-api/go.mod* internal/superphenix-api/go.sum* ./internal/superphenix-api/
COPY internal/superphenix-controller/go.mod* internal/superphenix-controller/go.sum* ./internal/superphenix-controller/
COPY pkg/chi-helper/go.mod* pkg/chi-helper/go.sum* ./pkg/chi-helper/
COPY pkg/permify-wrapper/go.mod* pkg/permify-wrapper/go.sum* ./pkg/permify-wrapper/
COPY pkg/superphenix-id/go.mod* pkg/superphenix-id/go.sum* ./pkg/superphenix-id/
COPY pkg/utils/go.mod* pkg/utils/go.sum* ./pkg/utils/
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the Go source (relies on .dockerignore to filter)
COPY . .

ARG VERSION=dev

# Build
# the GOARCH has no default value to allow the binary to be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
#RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -a -o manager cmd/superphenix-operator/main.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o manager -ldflags "-X 'github.com/super-phenix/superphenix/internal/superphenix-operator/version.OperatorVersion=${VERSION}'" cmd/superphenix-operator/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
