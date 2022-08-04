#!/usr/bin/env bash
set -eo pipefail
DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" &> /dev/null && pwd)"
cd "${DIR}"

kind delete cluster
kind create cluster --config=kind.yaml
kustomize build . | kubectl apply -f -
