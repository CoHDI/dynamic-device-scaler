ARG BUILD_IMAGE=docker.io/golang:1.24
ARG BASE_IMAGE=gcr.io/distroless/static:nonroot

# Build the manager binary
FROM $BUILD_IMAGE AS builder

WORKDIR /workspace

COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY cmd/main.go cmd/main.go
COPY internal/ internal/
RUN CGO_ENABLED=0 GO111MODULE=on go build -a -o dynamic-device-scaler cmd/main.go

# Copy the controller-manager into a thin image
# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM $BASE_IMAGE
WORKDIR /
COPY --from=builder /workspace/dynamic-device-scaler .
USER nonroot:nonroot
