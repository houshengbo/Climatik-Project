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
        --image-registry)
            IMAGE_REGISTRY="$2"
            shift 2
            ;;
        --image-tag)
            IMAGE_TAG="$2"
            shift 2
            ;;
        --prometheus-url)
            PROMETHEUS_URL="$2"
            shift 2
            ;;
        --monitor-interval)
            MONITOR_INTERVAL="$2"
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

# Create namespace if it doesn't exist
echo "Creating namespace ${NAMESPACE} if it doesn't exist..."
kubectl create namespace ${NAMESPACE} --dry-run=client -o yaml | kubectl apply -f -

# Install PowerCapping CRDs
echo "Installing PowerCapping CRDs..."
cd "${PROJECT_ROOT}/powercapping-controller"
kubectl apply -f config/crd/bases/

# Install FreqTuner CRDs
echo "Installing FreqTuner CRDs..."
cd "${PROJECT_ROOT}/freqtuner"
kubectl apply -f config/crd/bases/

# Deploy PowerCapping Controller
echo "Deploying PowerCapping Controller..."
cd "${PROJECT_ROOT}/powercapping-controller"
envsubst < "manifests/rbac/serviceaccount.yaml" | kubectl apply -f -
envsubst < "manifests/rbac/clusterrole.yaml" | kubectl apply -f -
envsubst < "manifests/rbac/clusterrolebinding.yaml" | kubectl apply -f -
envsubst < "manifests/powercapping-controller-deployment.yaml" | kubectl apply -f -

# Deploy FreqTuner
echo "Deploying FreqTuner..."
cd "${PROJECT_ROOT}/freqtuner"
envsubst < "manifests/rbac/serviceaccount.yaml" | kubectl apply -f -
envsubst < "config/rbac/role.yaml" | kubectl apply -f -
envsubst < "manifests/rbac/clusterrolebinding.yaml" | kubectl apply -f -
envsubst < "manifests/freqtuner-daemonset.yaml" | kubectl apply -f -

echo "Deployment complete!" 