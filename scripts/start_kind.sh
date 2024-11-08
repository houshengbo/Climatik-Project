#!/usr/bin/env bash
set -e
set -o pipefail

echo "> Creating temporary directory for nvidia-kind-deploy"
TMP_DIR=$(mktemp -d)
cd $TMP_DIR

echo "> Cloning nvidia-kind-deploy repository"
git clone https://github.com/SeineAI/nvidia-kind-deploy.git
cd nvidia-kind-deploy

echo "> Generating base kind configuration"
./kind-config-gen.sh --dry-run

echo "> Adding custom mounts to configuration"
# Insert custom mount before extraPortMappings using sed
sed -i '/extraPortMappings/i\  - hostPath: /data\n    containerPath: /data' kind-config.yaml

echo "> Creating Kind cluster and setting up GPU support"
make all

echo "> Validating cluster setup..."
# Check if nodes are ready
if ! kubectl get nodes | grep -q "Ready"; then
    echo "Error: Nodes are not in Ready state"
    exit 1
fi

# Check GPU operator status
if ! kubectl get pods -n gpu-operator | grep -q "Running"; then
    echo "Error: GPU operator pods are not running"
    exit 1
fi

# Test GPU availability
if ! kubectl get nodes -o json | grep -q "nvidia.com/gpu"; then
    echo "Error: GPU resources not available in the cluster"
    exit 1
fi

# Verify data directory mount
if ! docker exec kind-control-plane ls /data > /dev/null; then
    echo "Error: /data directory not mounted in the cluster"
    exit 1
fi

echo "> Cluster validation successful!"

echo "> Cleaning up nvidia-kind-deploy repository"
cd ../..
rm -rf $TMP_DIR

echo "> Kind cluster setup completed successfully!"

