#!/bin/bash

set -e

CLUSTER_NAME="url-shortener"

echo "Stopping port-forwarding processes..."
pkill -f "port-forward svc/envoy" || true
pkill -f "port-forward svc/envoy-read" || true
pkill -f "port-forward svc/signoz-frontend" || true

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

echo "Cleanup complete!"
