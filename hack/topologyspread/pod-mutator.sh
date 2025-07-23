#!/usr/bin/env bash

set -euo pipefail

REQUIRED_VARS=(CASTAI_API_URI ORGANIZATION_ID CLUSTER_ID CASTAI_API_TOKEN)
for var in "${REQUIRED_VARS[@]}"; do
  if [[ -z "${!var:-}" ]]; then
    echo "Error: Environment variable $var is not set." >&2
    exit 1
  fi
done


APP_NAMES=("echo-server" "echo-client")
HAS_CHANGES=false

for app_name in "${APP_NAMES[@]}"; do
  id=$(curl --request GET \
       --url "https://${CASTAI_API_URI}/patching-engine/v1beta/organizations/${ORGANIZATION_ID}/clusters/${CLUSTER_ID}/pod-mutations" \
       --silent \
       --header 'accept: application/json' \
       --header "authorization: Bearer ${CASTAI_API_TOKEN}" | \
       jq -r --arg name "$app_name" '.items[] | select(.name == $name) | .id')

  if [[ -n "$id" ]]; then
    echo "✅ Pod mutation for ${app_name} already exists with ID ${id}, skipping creation"
    continue
  fi

  HAS_CHANGES=true

  curl --request POST \
       --url "https://${CASTAI_API_URI}/patching-engine/v1beta/organizations/${ORGANIZATION_ID}/clusters/${CLUSTER_ID}/pod-mutations" \
       --silent \
       --header 'accept: application/json' \
       --header "authorization: Bearer ${CASTAI_API_TOKEN}" \
       --header 'content-type: application/json' \
       --data '{
           "objectFilter": {
               "namespaces": [
                   "taler"
               ],
               "kinds": [
                   "Deployment"
               ],
               "labelsFilter": [
                   {
                       "label": "app",
                       "value": "'"${app_name}"'"
                   }
               ]
           },
           "name": "'"${app_name}"'",
           "enabled": true,
           "annotations": {
             "linkerd.io/inject": "enabled"
           },
           "patch": [
               {
                   "op": "add",
                   "path": "/spec/topologySpreadConstraints",
                   "value": []
               },
               {
                   "op": "add",
                   "path": "/spec/topologySpreadConstraints/0",
                   "value": {
                       "labelSelector": {
                           "matchLabels": {
                               "app": "'"${app_name}"'"
                           }
                       },
                       "maxSkew": 1,
                       "topologyKey": "topology.kubernetes.io/zone",
                       "whenUnsatisfiable": "DoNotSchedule",
                       "matchLabelKeys": [
                        "pod-template-hash"
                       ]
                   }
               }
           ]
       }'
done

restart-all() {
  # The changes to pod mutation needs to be applied by the pod-mutator pod.
  # There seems to be a CRD for it but nothing is created, so this is the best
  # we can do. The loop in pod-mutator to update mutations is 30s
  echo "Waiting 30s for pod mutation to be applied..."
  sleep 30
  kubectl rollout restart deployment -n taler
  kubectl rollout restart statefulset -n taler
}


if [ "$HAS_CHANGES" = true ]; then
  echo "Changes detected, will restart deployments"
  restart-all
else
  echo "No changes detected, skipping restart"
fi