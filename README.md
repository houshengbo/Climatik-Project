# Dynamic Power Capping on Kubernetes

## 1. Overview

The purpose of this project is to design and implement a system that dynamically tunes GPU and CPU frequencies to achieve power capping for specific services or workloads in a Kubernetes environment. This system is particularly tailored for GPU-intensive tasks such as Large Language Model (LLM) inference services and LLM training workloads.

Key features of the system include:

1. **Dynamic Power Management**: The system continuously monitors power usage and adjusts GPU and CPU frequencies in real-time to maintain power consumption within specified limits.

2. **Workload-Specific Policies**: Different power caps can be set for various services or workloads, allowing fine-grained control over power consumption across the cluster.

3. **Kubernetes-Native Design**: The system is fully integrated with Kubernetes, using Custom Resources (CRs) and custom controllers to manage power capping policies and frequency adjustments.

4. **Flexible Algorithm Integration**: The system supports custom algorithms for determining frequency adjustments, allowing for sophisticated power management strategies.

The architecture consists of three main components:

1. **Power Usage Monitor**: Continuously monitors power consumption using data from DCGM Exporter via Prometheus, implementing a `powercapping-controller` to monitor the power usage of all resources for a service / deployment.

2. **Action Recommender**: Analyzes power usage data and recommends frequency scaling actions based on defined policies, implementing a `freqtuning-recommender` to recommend frequency scaling actions.

3. **Frequency Tuner DaemonSet**: Applies the recommended frequency changes on individual nodes, implementing a `freqtuner` to apply the recommended frequency changes on individual nodes.

These components work together to ensure that power-intensive workloads like LLM inference and training can operate efficiently within specified power constraints. The system uses two Custom Resources:

1. **PowerCappingPolicy**: Defines the power capping policy for a specific workload or service.
2. **NodeFrequencies**: Manages the frequency settings for GPUs and CPUs on specific nodes.

By dynamically adjusting GPU and CPU frequencies based on real-time power consumption data, this system enables organizations to maximize the performance of their LLM workloads while staying within power budget constraints. This approach is particularly valuable in environments where power efficiency is crucial, such as large-scale AI training clusters or edge computing scenarios running inference services.

## 2. Motivation

The motivation for this project is to address the challenges of power management in large-scale Kubernetes environments, particularly for GPU-intensive workloads like LLM inference and training. Traditional power management solutions often lack the flexibility and granularity required to optimize power consumption for these workloads. This project aims to fill this gap by providing a dynamic power capping service that can be easily integrated into existing Kubernetes clusters.

Key benefits of the PowerCappingFreqTuner project include:

1. **Power Efficiency**: By dynamically adjusting GPU and CPU frequencies, the system can significantly reduce power consumption, leading to cost savings and environmental benefits.

2. **Performance Optimization**: The system ensures that power-intensive workloads like LLM inference and training can operate efficiently within specified power constraints, maximizing performance while staying within power budget constraints.

3. **Flexibility**: The system is designed to be highly flexible, allowing for easy integration into existing Kubernetes clusters and customization of power management policies.

## 3. Architecture

The Climatik Project implements a dynamic power capping service using Kubernetes. The architecture consists of several key components:

1. Power Usage Monitor (`powercapping-controller`): A custom Kubernetes controller that monitors power usage and determines if capping is needed. It reads from and updates the PowerCappingPolicy CR, and receives data from Prometheus (fed by DCGM Exporter).
2. Action Recommender (`freqtuning-recommender`): Recommends scaling actions based on the power capping policy. It reads from the PowerCappingPolicy CR and creates/updates the NodeFrequencies CR with recommended actions.
3. Frequency Tuner DaemonSet (`freqtuner`): Applies frequency changes on individual nodes. It reads from the NodeFrequencies CR and updates its status after applying changes.

The system uses Custom Resources (CRs) to define power capping policies and manage node frequencies, providing a flexible and scalable approach to power management in Kubernetes clusters:

1. PowerCappingPolicy: A Custom Resource (CR) that defines the power capping policy for a specific workload or service. 
2. NodeFrequencies: A Custom Resource (CR) that manages the frequency settings for GPUs and CPUs on specific nodes. 

This architecture allows for dynamic power management, workload-specific policies, flexible algorithm integration, and seamless integration with Kubernetes environments, making it particularly useful for GPU-intensive workloads like LLM inference and training.

## 4 Prerequisites

- go version v1.22.0+
- docker version 17.03+
- kubectl version v1.11.3+
- Kind (Kubernetes in Docker) installed
- (Optional) Access to a machine with NVIDIA GPUs for GPU-enabled testing

### Setting up the Kubernetes Cluster

#### For CPU-only machines:

1. Create a Kind cluster:
   ```sh
   kind create cluster --name powercapping-cluster
   ```

2. Verify the cluster is running:
   ```sh
   kubectl cluster-info --context kind-powercapping-cluster
   ```

#### For GPU-enabled machines:

1. Ensure you have the NVIDIA Container Toolkit installed on your host machine.

2. Use the provided script to create a Kind cluster with GPU support:
   ```sh
   ./scripts/start_kind.sh
   ```

   This script will:
   - Create a Kind cluster with the necessary configurations for GPU support
   - Deploy cert-manager
   - Install the NVIDIA GPU Operator
   - Configure custom device plugin settings

3. Verify the cluster and GPU support:
   ```sh
   kubectl get nodes
   kubectl get pods -n gpu-operator


## 5. Installation

### 5.1 Quick Installation

The easiest way to install the PowerCapping Controller is using our deployment script:

```sh
./scripts/deploy.sh
```

### 5.2 Verification

Verify the installation by checking the status of the deployed components:

```sh
# Check if controllers are running
kubectl get pods -n climatik-project

# Verify CRDs are installed
kubectl get crds | grep climatik.io
```

### 5.3 Uninstallation

To remove all components from your cluster:

```sh
./scripts/undeploy.sh
```

### 5.4 Manual Installation

If you prefer to install components manually or customize the installation, refer to our [detailed installation guide](docs/manual-installation.md).

### 5.5 Troubleshooting

If you encounter any issues during installation:

1. Check the controller logs:
   ```sh
   kubectl logs -n climatik-project -l app=powercapping-controller
   ```

2. Verify RBAC permissions:
   ```sh
   kubectl auth can-i --as system:serviceaccount:climatik-project:powercapping-controller -n default get powercappingpolicies.climatik.io
   kubectl auth can-i --as system:serviceaccount:climatik-project:powercapping-controller -n default patch powercappingpolicies.climatik.io/status
   ```

3. Ensure all CRDs are properly installed:
   ```sh
   kubectl get crd powercappingpolicies.climatik.io
   kubectl get crd nodefrequencies.climatik.io
   ```

For more detailed troubleshooting steps, please refer to our [troubleshooting guide](docs/troubleshooting.md).

## 6. Usage

### Deploying the VLLM Deployment with OPT-125M Model

Follow these steps to deploy the VLLM server with the `facebook/opt-125m` model on your Kubernetes cluster:

1. **Create the Hugging Face Token Secret**

   First, create a Kubernetes secret to store your Hugging Face token. Replace `<hg_token>` with your actual Hugging Face token:

   ```sh
   kubectl create secret generic huggingface-secret --from-literal=HF_TOKEN='<hg_token>'
   ```

2. **Prepare the Environment**

   Create and set permissions for the cache directory:
   ```sh
   sudo mkdir -p /data/huggingface-cache
   sudo chmod 777 /data/huggingface-cache
   ```

3. **Load the VLLM Image into Kind Cluster**

   ```sh
   # Pull the image locally
   docker pull vllm/vllm-openai:latest
   
   # Load the image into kind cluster
   kind load docker-image vllm/vllm-openai:latest
   ```

4. **Deploy the VLLM Server**

   ```sh
   kubectl apply -f manifests/vllm-deployment.yaml
   ```

5. **Verify the Deployment**

   ```sh
   kubectl get pods -l app=vllm-opt-125m
   ```

   You should see a pod running with the name `vllm-opt-125m`.

4. **Access the VLLM Server**

   Once the pod is running, you can access the VLLM server using the service created in the deployment:

   ```sh
   kubectl get svc vllm-opt-125m
   ```

   This will provide you with the service's external IP and port, which you can use to interact with the VLLM server.

By following these steps, you will have successfully deployed the VLLM server with the `facebook/opt-125m` model on your Kubernetes cluster, utilizing persistent volumes for efficient model caching.

## 7. Documentation

For a detailed description of the system architecture, including component interactions and workflow, please refer to our [design document](docs/design.md). This document provides:

- A system architecture diagram
- Detailed descriptions of Custom Resources (CRs)
- Explanations of the main controllers and their functions
- The overall system workflow
- Key benefits of the architecture

The design document offers a comprehensive overview of how the PowerCapping Controller works in conjunction with other components to achieve efficient power management for LLM inference workloads.

## 8. Contributing

Contributions to the project are welcome! If you find any issues or have suggestions for improvement, please open an issue or submit a pull request on the GitHub repository.

For detailed information on how to contribute to this project, please refer to our [CONTRIBUTING.md](CONTRIBUTING.md) file.

**NOTE:** Run `make help` for more information on all potential `make` targets.

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html).

## 9. License

This project is licensed under the Apache License 2.0. For full details, see the [LICENSE](LICENSE) file.

## 10. Code of Conduct

The Climatik Project follows the [CNCF Code of Conduct](code-of-conduct.md).

## 11. Maintainers

For a list of project maintainers and their contact information, please see our [MAINTAINERS.md](MAINTAINERS.md) file.

## 12. Contact

For any questions or inquiries, please contact the project maintainers listed in [MAINTAINERS.md](MAINTAINERS.md).