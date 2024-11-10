#!/bin/bash
set -e

# Default values
NAMESPACE="climatik-project"
IMAGE_REGISTRY="quay.io/climatik-project"
IMAGE_TAG="latest"
PROMETHEUS_URL="http://prometheus-k8s.monitoring:9090"
MONITOR_INTERVAL="1m"

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

# Export variables for envsubst
export NAMESPACE IMAGE_REGISTRY IMAGE_TAG PROMETHEUS_URL MONITOR_INTERVAL

# Get the project root directory
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Remove FreqTuner
echo "Removing FreqTuner..."
cd "${PROJECT_ROOT}/freqtuner"
if [ -f "manifests/freqtuner-daemonset.yaml" ]; then
    envsubst < "manifests/freqtuner-daemonset.yaml" | kubectl delete -f - --ignore-not-found || true
fi
if [ -f "manifests/rbac/clusterrolebinding.yaml" ]; then
    envsubst < "manifests/rbac/clusterrolebinding.yaml" | kubectl delete -f - --ignore-not-found || true
fi
if [ -f "config/rbac/role.yaml" ]; then
    envsubst < "config/rbac/role.yaml" | kubectl delete -f - --ignore-not-found || true
fi
if [ -f "manifests/rbac/serviceaccount.yaml" ]; then
    envsubst < "manifests/rbac/serviceaccount.yaml" | kubectl delete -f - --ignore-not-found || true
fi

# Remove PowerCapping Controller
echo "Removing PowerCapping Controller..."
cd "${PROJECT_ROOT}/powercapping-controller"
if [ -f "manifests/powercapping-controller-deployment.yaml" ]; then
    envsubst < "manifests/powercapping-controller-deployment.yaml" | kubectl delete -f - --ignore-not-found || true
fi
if [ -f "manifests/rbac/clusterrolebinding.yaml" ]; then
    envsubst < "manifests/rbac/clusterrolebinding.yaml" | kubectl delete -f - --ignore-not-found || true
fi
if [ -f "manifests/rbac/clusterrole.yaml" ]; then
    envsubst < "manifests/rbac/clusterrole.yaml" | kubectl delete -f - --ignore-not-found || true
fi
if [ -f "manifests/rbac/serviceaccount.yaml" ]; then
    envsubst < "manifests/rbac/serviceaccount.yaml" | kubectl delete -f - --ignore-not-found || true
fi

# Remove all NodeFrequencies CRs from all namespaces
echo "Removing NodeFrequencies CRs from all namespaces..."
kubectl get nodefrequencies.climatik.io --all-namespaces -o name | xargs -r kubectl delete --ignore-not-found || true

# Remove all PowerCappingPolicies CRs from all namespaces
echo "Removing PowerCappingPolicies CRs from all namespaces..."
kubectl get powercappingpolicies.climatik.io --all-namespaces -o name | xargs -r kubectl delete --ignore-not-found || true

# Wait for CRs to be deleted
echo "Waiting for CRs to be deleted..."
sleep 5

# Remove FreqTuner CRDs
echo "Removing FreqTuner CRDs..."
cd "${PROJECT_ROOT}/freqtuner"
if [ -d "config/crd/bases" ]; then
    kubectl delete -f config/crd/bases/ --ignore-not-found || true
fi

# Remove PowerCapping CRDs
echo "Removing PowerCapping CRDs..."
cd "${PROJECT_ROOT}/powercapping-controller"
if [ -d "config/crd/bases" ]; then
    kubectl delete -f config/crd/bases/ --ignore-not-found || true
fi

# Remove FreqTuning Recommender
echo "Removing FreqTuning Recommender..."
cd "${PROJECT_ROOT}/freqtuning-recommender"
if [ -f "manifests/freqtuning-recommender-deployment.yaml" ]; then
    envsubst < "manifests/freqtuning-recommender-deployment.yaml" | kubectl delete -f - --ignore-not-found || true
fi
if [ -f "manifests/rbac/clusterrolebinding.yaml" ]; then
    envsubst < "manifests/rbac/clusterrolebinding.yaml" | kubectl delete -f - --ignore-not-found || true
fi
if [ -f "manifests/rbac/clusterrole.yaml" ]; then
    envsubst < "manifests/rbac/clusterrole.yaml" | kubectl delete -f - --ignore-not-found || true
fi
if [ -f "manifests/rbac/serviceaccount.yaml" ]; then
    envsubst < "manifests/rbac/serviceaccount.yaml" | kubectl delete -f - --ignore-not-found || true
fi

# Remove namespace if empty
echo "Removing namespace..."
kubectl delete namespace ${NAMESPACE} --ignore-not-found || true

echo "Undeployment complete!"