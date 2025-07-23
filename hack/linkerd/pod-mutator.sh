#!/usr/bin/env bash

set -euo pipefail

REQUIRED_VARS=(CASTAI_API_URI ORGANIZATION_ID CLUSTER_ID CASTAI_API_TOKEN)
for var in "${REQUIRED_VARS[@]}"; do
  if [[ -z "${!var:-}" ]]; then
    echo "Error: Environment variable $var is not set." >&2
    exit 1
  fi
done

restart-all() {
  # The changes to pod mutation needs to be applied by the pod-mutator pod.
  # There seems to be a CRD for it but nothing is created, so this is the best
  # we can do. The loop in pod-mutator to update mutations is 30s
  echo "Waiting 30s for pod mutation to be applied..."
  sleep 30
  kubectl rollout restart deployment -n taler echo-client echo-server
}

operation="${1:-}"

id=$(curl --request GET \
     --url "https://${CASTAI_API_URI}/patching-engine/v1beta/organizations/${ORGANIZATION_ID}/clusters/${CLUSTER_ID}/pod-mutations" \
     --header 'accept: application/json' \
     --header "authorization: Bearer ${CASTAI_API_TOKEN}" --silent | \
     jq -r '.items[] | select(.name == "taler-hazl-mutation") | .id')

if [[ "$operation" == "remove" ]];then
  if  [[ -z "$id" ]]; then
    echo "Pod mutation already removed"
    exit 0
  fi

  curl --request DELETE \
     --url "https://${CASTAI_API_URI}/patching-engine/v1beta/organizations/${ORGANIZATION_ID}/clusters/${CLUSTER_ID}/pod-mutations/${id}" \
     --header 'accept: application/json' \
     --header "authorization: Bearer ${CASTAI_API_TOKEN}" \
     --silent

  restart-all
  exit $?
fi

if [[ -n "$id" ]]; then
  echo "✅ Pod mutation already installed"
  exit 0
fi

curl --request POST --silent \
     --url "https://${CASTAI_API_URI}/patching-engine/v1beta/organizations/${ORGANIZATION_ID}/clusters/${CLUSTER_ID}/pod-mutations" \
     --header 'accept: application/json' \
     --header "authorization: Bearer ${CASTAI_API_TOKEN}" \
     --header 'content-type: application/json' \
     --data '
{
  "objectFilter": {
    "namespaces": [
      "taler"
    ]
  },
  "annotations": {
    "linkerd.io/inject": "enabled"
  },
  "name": "taler-hazl-mutation",
  "enabled": true
}
'

restart-all
