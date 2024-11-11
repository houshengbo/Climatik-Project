# KubeCon NA 2024 - Climatik Demo

### Preparation: Setup and Verify Environment
1. Create a kind cluster and verify the GPU operator and GPU nodes are ready
```bash
kubectl get nodes
kubectl get pods -n gpu-operator
```
*Purpose: Verify the available nodes in your Kubernetes cluster*

2. Verify the Observability stack is ready
```bash
kubectl get pods -n monitoring
```

3. Forward the Grafana service to your local machine
```bash
kubectl --namespace monitoring port-forward svc/grafana 3000:3000 --address 0.0.0.0
```
*Expected output: `Forwarding from 0.0.0.0:3000 -> 3000`*

4. Check GPU configuration
```bash
# Check GPU metrics (current frequencies)
nvidia-smi --query-gpu=gpu_name,clocks.current.graphics,clocks.current.memory --format=csv
```
*Purpose: Ensure GPU is configured correctly*

### Clone the code and Install Climatik
1. Clone the Climatik repository and checkout the KubeCon NA branch
```bash
git clone https://github.com/sustainable-computing-io/climatik.git
cd climatik
git checkout kubeconNA
```

2. Install the Climatik controllers
```bash
./scripts/deploy.sh
```

3. Verify the installation
```bash
kubectl get pods -n climatik-project
```
*Expected output:*
```
NAME                                       READY   STATUS    RESTARTS   AGE
freqtuner-j5bb6                            1/1     Running   0          4m22s
freqtuning-recommender-86479468b4-fj9mn    1/1     Running   0          4m21s
powercapping-controller-75555c9cf6-p4g5r   1/1     Running   0          4m22s
```

4. Watch the Climatik controllers in action
```bash
# Open these commands in separate terminal windows
# Terminal 1: Watch the powercapping controller
kubectl logs -f powercapping-controller-75555c9cf6-p4g5r -n climatik-project

# Terminal 2: Watch the frequency tuning recommender
kubectl logs -f freqtuning-recommender-86479468b4-fj9mn -n climatik-project

# Terminal 3: Watch the frequency tuner
kubectl logs -f freqtuner-j5bb6 -n climatik-project
```
*Purpose: Monitor the real-time logs of all Climatik controllers*

5. Deploy the Climatik demo load testing application

First, make sure a vLLM server is running for LLM inference workload:
```bash
kubectl get pods
```
*Expected output:*
```
NAME                             READY   STATUS    RESTARTS   AGE
vllm-opt-125m-757564ffcf-8rv7s   1/1     Running   0          2d23h
```

Then, deploy the load testing application:
```bash
./benchmarks/benchmark-runner.sh
```
*Purpose: Deploy a load tester to stress the LLM server*

Verify the testing load is running and stressing the server:
```bash
# Check the job status
kubectl get jobs
```
*Expected output:*
```
NAME                 COMPLETIONS   DURATION   AGE
vllm-benchmark-job   0/1           13s        13s
```

```bash
# Check the running pods
kubectl get pods
```
*Expected output:*
```
NAME                             READY   STATUS    RESTARTS   AGE
vllm-benchmark-job-h8tsr         1/1     Running   0          21s
vllm-opt-125m-757564ffcf-8rv7s   1/1     Running   0          2d23h
```

### Deploy and Monitor PowerCappingPolicy
1. Deploy the PowerCappingPolicy for the vLLM server
```bash
kubectl create -f manifests/powercappingpolicy-sample.yaml
```

2. Verify the PowerCappingPolicy deployment
```bash
kubectl get powercappingpolicies
```
*Expected output should show `llm-inference-power-cap` policy*

3. Monitor the effects
```bash
# Check the PowerCappingPolicy details
kubectl get powercappingpolicies llm-inference-power-cap -oyaml

# Check the NodeFrequencies CR status
kubectl get nf kind-control-plane -n climatik-project -oyaml
```

*Purpose: The PowerCappingPolicy will trigger:*
- *PowerCapping controller to monitor power usage*
- *FreqTuning Recommender to suggest optimal frequencies*
- *FreqTuner to update NodeFrequencies CR and apply the changes*

*Monitor these effects in the controller logs opened in the previous step*

### Reset GPU Configuration (Optional)
1. Reset GPU frequencies to default values
```bash
# Reset GPU 0
sudo nvidia-smi -i 0 -rgc
sudo nvidia-smi -i 0 -rmc

# Reset GPU 1
sudo nvidia-smi -i 1 -rgc
sudo nvidia-smi -i 1 -rmc
```

2. Verify the frequency reset
```bash
nvidia-smi --query-gpu=gpu_name,clocks.current.graphics,clocks.current.memory --format=csv
```
*Purpose: Return GPU frequencies to their default settings after testing. The `-rgc` flag resets graphics clocks and `-rmc` resets memory clocks to their default values.*
