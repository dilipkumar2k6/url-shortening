#!/bin/bash

set -e

CLUSTER_NAME="url-shortener"

echo "Stopping port-forwarding processes..."
pkill -f "port-forward svc/envoy -n istio-system" || true
pkill -f "port-forward svc/envoy-read -n istio-system" || true
pkill -f "port-forward svc/signoz-frontend" || true
pkill -f "port-forward deployment/write-api" || true
pkill -f "port-forward deployment/analytics-api" || true

if kind get clusters | grep -q "^${CLUSTER_NAME}$"; then
    echo "Exporting kubeconfig for kind cluster..."
    KUBECONFIG_FILE="$HOME/.kube/kind-${CLUSTER_NAME}"
    kind get kubeconfig --name ${CLUSTER_NAME} > "$KUBECONFIG_FILE"
    export KUBECONFIG="$KUBECONFIG_FILE"

    echo "Deleting Kubernetes resources..."
    kubectl delete -R -f k8s/ --ignore-not-found

    echo "Deleting kind cluster..."
    kind delete cluster --name ${CLUSTER_NAME}
    rm -f "$HOME/.kube/kind-${CLUSTER_NAME}"
else
    echo "Kind cluster ${CLUSTER_NAME} does not exist."
fi

echo "Cleaning up local Istio files..."
rm -rf istio-*/

echo "Cleanup complete!"
