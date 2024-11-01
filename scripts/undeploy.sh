#!/bin/bash
set -e

NAMESPACE="climatik-project"

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Get the directory containing the project root (where Makefile is)
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Remove the deployment
echo "Removing deployment..."
kubectl delete deployment powercapping-controller -n ${NAMESPACE} --ignore-not-found

# Remove RBAC of powercapping-controller
echo "Removing RBAC of powercapping-controller..."
kubectl delete -f "${PROJECT_ROOT}/powercapping-controller/manifests/rbac/clusterrolebinding.yaml" --ignore-not-found
kubectl delete -f "${PROJECT_ROOT}/powercapping-controller/manifests/rbac/clusterrole.yaml" --ignore-not-found
kubectl delete -f "${PROJECT_ROOT}/powercapping-controller/manifests/rbac/serviceaccount.yaml" --ignore-not-found

# Remove CRD of powercapping-controller
echo "Removing CRD of powercapping-controller..."
cd "${PROJECT_ROOT}/powercapping-controller"
make uninstall
cd - > /dev/null

# Remove namespace if empty
echo "Removing namespace..."
kubectl delete namespace ${NAMESPACE} --ignore-not-found --wait=true

echo "Undeployment complete!"