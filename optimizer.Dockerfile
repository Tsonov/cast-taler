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

# Use an intermediate stage to download and install linkerd CLI
FROM --platform=linux/amd64 alpine:3.18 AS linkerd-installer
RUN apk add --no-cache curl
# Install linkerd CLI
RUN curl --proto '=https' --tlsv1.2 -sSfL https://enterprise.buoyant.io/install | sh
# The linkerd binary is installed to $HOME/.linkerd2/bin
RUN mkdir -p /linkerd-bin
RUN cp $HOME/.linkerd2/bin/linkerd /linkerd-bin/

FROM gcr.io/distroless/static-debian12:latest

# Copy the built binary from the builder stage
COPY --from=builder --chown=nonroot:nonroot /optimizer /optimizer

# Copy the linkerd CLI from the linkerd-installer stage
COPY --from=linkerd-installer --chown=nonroot:nonroot /linkerd-bin/linkerd /usr/local/bin/linkerd

# Copy all contents from the hack directory
COPY hack/ /hack/

USER nonroot:nonroot
ENTRYPOINT ["/optimizer"]
