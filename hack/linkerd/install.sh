#!/usr/bin/env bash

set -euo pipefail

# Check that the current k8s context does not contain 'prod' or 'dev'
CURRENT_CONTEXT=$(kubectl config current-context)
if [[ "$CURRENT_CONTEXT" == *prod* ]] || [[ "$CURRENT_CONTEXT" == *dev* ]]; then
  echo "Error: Current Kubernetes context ('$CURRENT_CONTEXT') contains 'prod' or 'dev'. Aborting."
  exit 1
fi

# Check that 'castai-cluster-controller' is running
if ! kubectl get pods -n castai-agent --no-headers | grep -q 'castai-cluster-controller.*Running'; then
  echo "Error: No 'castai-cluster-controller' pod is in Running state in the 'castai-agent' namespace."
  echo "Please onboard the cluster to CAST AI phase 2 or check if there are issues with the controller pod."
  exit 1
fi

# Validate that BUOYANT_LICENSE is set
if [[ -z "$BUOYANT_LICENSE" ]]; then
  echo "Error: BUOYANT_LICENSE environment variable is not set."
  echo "Please go to 1password and check for the 'Buoyant Enterprise trial key' note."
  exit 1
fi

# Check if Gateway API CRDs are already installed with appropriate version
if ! kubectl get crds/httproutes.gateway.networking.k8s.io -o "jsonpath={.metadata.annotations.gateway\.networking\.k8s\.io/bundle-version}" &> /dev/null; then
  echo "Installing Gateway API CRDs..."
  kubectl apply -f https://github.com/kubernetes-sigs/gateway-api/releases/download/v1.2.1/standard-install.yaml
else
  echo "Gateway API CRDs already installed, skipping..."
fi

# Check if Linkerd CLI is already installed
if ! command -v linkerd &> /dev/null; then
  echo "Installing Linkerd CLI..."
  curl --proto '=https' --tlsv1.2 -sSfL https://enterprise.buoyant.io/install | sh

  # Add Linkerd to PATH for current session and suggest adding to profile
  export PATH="$HOME/.linkerd2/bin:$PATH"
  echo "Linkerd CLI installed to $HOME/.linkerd2/bin"
  echo "For permanent access, add this to your shell profile:"
  echo "  export PATH=\"\$HOME/.linkerd2/bin:\$PATH\""
else
  echo "Linkerd CLI already installed, skipping..."
fi

# Detect an existing, healthy control-plane ----------------------------
if kubectl get ns linkerd &>/dev/null; then
  if linkerd check --wait 0 --output short &>/dev/null; then
    echo "✅ Linkerd already installed and healthy – skipping installation."
    exit 0
  else
    echo "⚠️  Linkerd present but not healthy. Investigate or run 'linkerd upgrade'."
    exit 1
  fi
fi


# Get cluster network configuration from GKE
CLUSTER_NAME=$(kubectl config current-context | cut -d '_' -f 4)
PROJECT_ID=$(kubectl config current-context | cut -d '_' -f 2)
REGION=$(gcloud container clusters list --filter="name=${CLUSTER_NAME}" --project=${PROJECT_ID} --format="value(location)")
CLUSTER_CIDR=$(gcloud container clusters describe ${CLUSTER_NAME} --region=${REGION} --project=${PROJECT_ID} --format="value(clusterIpv4Cidr)")
SERVICES_CIDR=$(gcloud container clusters describe ${CLUSTER_NAME} --region=${REGION} --project=${PROJECT_ID} --format="value(servicesIpv4Cidr)")
POD_CIDR="10.0.0.0/8"
NETWORK_NAME=$(gcloud container clusters describe ${CLUSTER_NAME} --region=${REGION} --project=${PROJECT_ID} --format="value(network)")
VPC_CIDR=$(gcloud compute networks subnets list --filter="region:${REGION}" --project=${PROJECT_ID} | grep " ${NETWORK_NAME} " | awk '{print $3}' | xargs -I{} gcloud compute networks subnets describe {} --region=${REGION} --project=${PROJECT_ID} --format="value(ipCidrRange)" | sed 's/$/\\,/' | tr -d '\n')

# Build the clusterNetworks parameter
CLUSTER_NETWORKS="${CLUSTER_CIDR}\\,${SERVICES_CIDR}\\,${POD_CIDR}\\,${VPC_CIDR}fd00::/8"
echo "Using dynamically detected cluster networks: ${CLUSTER_NETWORKS}"


linkerd version --client
linkerd check --pre
linkerd install --crds | kubectl apply -f -
linkerd install --ha --set "clusterNetworks=${CLUSTER_NETWORKS}" | kubectl apply -f -
linkerd check
