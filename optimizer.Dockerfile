FROM --platform=linux/amd64 ghcr.io/castai/live/golang:1.24.4-alpine AS builder

WORKDIR /src

ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

ENV GOCACHE=/go-cache
ENV GOMODCACHE=/gomod-cache

COPY optimizer/go.mod .
COPY optimizer/go.sum .
RUN go mod download

COPY optimizer/ .

# Build the optimizer application
RUN --mount=type=cache,target=/go-cache \
    go build -o /optimizer -ldflags="-s -w" .

FROM gcr.io/distroless/static-debian12:latest

# Copy the built binary from the builder stage
COPY --from=builder --chown=nonroot:nonroot /optimizer /optimizer

# Copy all contents from the hack directory
COPY hack/ /hack/

USER nonroot:nonroot
ENTRYPOINT ["/optimizer"]