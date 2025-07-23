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

# Use an intermediate stage to download and install linkerd CLI, kubectl, and gcloud CLI
FROM --platform=linux/amd64 alpine:3.18 AS tools-installer
RUN apk add --no-cache curl bash python3

# Install linkerd CLI
RUN curl --proto '=https' --tlsv1.2 -sSfL https://enterprise.buoyant.io/install | sh
# The linkerd binary is installed to $HOME/.linkerd2/bin
RUN mkdir -p /tools-bin
RUN cp $HOME/.linkerd2/bin/linkerd /tools-bin/

# Install kubectl
RUN curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl" && \
    chmod +x kubectl && \
    mv kubectl /tools-bin/

# Install gcloud CLI
RUN curl -L -O https://dl.google.com/dl/cloudsdk/channels/rapid/downloads/google-cloud-cli-446.0.1-linux-x86_64.tar.gz && \
    tar -xzf google-cloud-cli-446.0.1-linux-x86_64.tar.gz && \
    ./google-cloud-sdk/install.sh --quiet --usage-reporting=false --path-update=false && \
    cp ./google-cloud-sdk/bin/gcloud /tools-bin/ && \
    cp -r ./google-cloud-sdk/lib /tools-bin/ && \
    cp -r ./google-cloud-sdk/platform /tools-bin/

FROM debian:bullseye-slim

# Install Python for gcloud CLI
RUN apt-get update && apt-get install -y python3 && apt-get clean

# Create nonroot user and group
RUN groupadd -r nonroot && useradd -r -g nonroot nonroot

# Copy the built binary from the builder stage
COPY --from=builder --chown=nonroot:nonroot /optimizer /optimizer

# Copy the tools from the tools-installer stage
COPY --from=tools-installer --chown=nonroot:nonroot /tools-bin/linkerd /usr/local/bin/linkerd
COPY --from=tools-installer --chown=nonroot:nonroot /tools-bin/kubectl /usr/local/bin/kubectl
COPY --from=tools-installer --chown=nonroot:nonroot /tools-bin/gcloud /usr/local/bin/gcloud
COPY --from=tools-installer --chown=nonroot:nonroot /tools-bin/lib /usr/local/lib/gcloud
COPY --from=tools-installer --chown=nonroot:nonroot /tools-bin/platform /usr/local/platform/gcloud

# Set up environment for gcloud CLI
ENV PATH="/usr/local/bin:${PATH}"
ENV CLOUDSDK_PYTHON="/usr/bin/python3"

# Copy all contents from the hack directory
COPY hack/ /hack/

USER nonroot:nonroot
ENTRYPOINT ["/optimizer"]
