#!/usr/bin/env bash

set -euo pipefail

# Check that the current k8s context does not contain 'prod' or 'dev'
CURRENT_CONTEXT=$(kubectl config current-context)
if [[ "$CURRENT_CONTEXT" == *prod* ]] || [[ "$CURRENT_CONTEXT" == *dev* ]]; then
  echo "Error: Current Kubernetes context ('$CURRENT_CONTEXT') contains 'prod' or 'dev'. Aborting."
  exit 1
fi

# Validate that BUOYANT_LICENSE is set
if [[ -z "$BUOYANT_LICENSE" ]]; then
  echo "Error: BUOYANT_LICENSE environment variable is not set."
  echo "Please go to 1password and check for the 'Buoyant Enterprise trial key' note."
  exit 1
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

linkerd uninstall | kubectl delete -f -
