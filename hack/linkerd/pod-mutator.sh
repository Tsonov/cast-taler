#!/usr/bin/env bash

set -euo pipefail

id=$(curl --request GET \
     --url "https://api.dev-master.cast.ai/patching-engine/v1beta/organizations/${ORGANIZATION_ID}/clusters/${CLUSTER_ID}/pod-mutations" \
     --header 'accept: application/json' \
     --header "authorization: Bearer ${CASTAI_API_TOKEN}" | \
     jq -r '.items[] | select(.name == "taler-hazl-mutation") | .id')

if [[ -n "$id" ]];then
  curl --request DELETE \
     --url "https://api.dev-master.cast.ai/patching-engine/v1beta/organizations/${ORGANIZATION_ID}/clusters/${CLUSTER_ID}/pod-mutations/${id}" \
     --header 'accept: application/json' \
     --header "authorization: Bearer ${CASTAI_API_TOKEN}"
fi

curl --request POST \
     --url "https://api.dev-master.cast.ai/patching-engine/v1beta/organizations/${ORGANIZATION_ID}/clusters/${CLUSTER_ID}/pod-mutations" \
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
