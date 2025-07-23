#!/usr/bin/env bash

set -euo pipefail

REQUIRED_VARS=(CASTAI_API_URI ORGANIZATION_ID CLUSTER_ID CASTAI_API_TOKEN)
for var in "${REQUIRED_VARS[@]}"; do
  if [[ -z "${!var:-}" ]]; then
    echo "Error: Environment variable $var is not set." >&2
    exit 1
  fi
done

id=$(curl --request GET \
     --url "https://${CASTAI_API_URI}/patching-engine/v1beta/organizations/${ORGANIZATION_ID}/clusters/${CLUSTER_ID}/pod-mutations" \
     --header 'accept: application/json' \
     --header "authorization: Bearer ${CASTAI_API_TOKEN}" | \
     jq -r '.items[] | select(.name == "taler-hazl-mutation") | .id')

if [[ -n "$id" ]];then
  curl --request DELETE \
     --url "https://${CASTAI_API_URI}/patching-engine/v1beta/organizations/${ORGANIZATION_ID}/clusters/${CLUSTER_ID}/pod-mutations/${id}" \
     --header 'accept: application/json' \
     --header "authorization: Bearer ${CASTAI_API_TOKEN}"
fi

curl --request POST \
     --url "https://${CASTAI_API_URI}/patching-engine/v1beta/organizations/${ORGANIZATION_ID}/clusters/${CLUSTER_ID}/pod-mutations" \
     --header 'accept: application/json' \
     --header "authorization: Bearer ${CASTAI_API_TOKEN}" \
     --header 'content-type: application/json' \
     --data '
{
  "objectFilter": {
    "namespaces": [
      "taler"
    ],
    "kinds": [
      "Pod"
    ]
  },
  "annotations": {
    "linkerd.io/inject": "enabled"
  },
  "name": "taler-hazl-mutation",
  "enabled": true
}
'
