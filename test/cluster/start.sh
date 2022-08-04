#!/usr/bin/env bash
set -eo pipefail
DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"
cd "${DIR}"

kind delete cluster
kind create cluster --config=kind.yaml
kubectl -n kube-system create secret generic kube-google-safebrowsing --from-literal=google-safebrowsing-api-key=${GOOGLE_SAFEBROWSING_API_KEY}
kustomize build . | kubectl apply -f -
