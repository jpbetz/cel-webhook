#!/usr/bin/env bash
set -o errexit
set -o nounset
set -o pipefail

scriptDir=$(dirname "${BASH_SOURCE[0]}")

pushd "${scriptDir}" > /dev/null
openssl req -x509 -newkey rsa:4096 -keyout webhook.key -out webhook.crt -days 365 -nodes \
    -subj "/CN=localhost" \
    -config <(cat /etc/ssl/openssl.cnf \
        <(printf "\n[SAN]\nsubjectAltName=DNS:localhost")) \
    -extensions SAN
popd > /dev/null

caBundle=$(base64 "${scriptDir}/webhook.crt" | awk 'BEGIN{ORS="";} {print}')
sed "s/CA_BUNDLE/${caBundle}/" "${scriptDir}/webhook-template.yaml" > "${scriptDir}/webhook.yaml"
sed "s/CA_BUNDLE/${caBundle}/" "${scriptDir}/crd-template.yaml" > "${scriptDir}/crd.yaml"