# vLLM Benchmarking Tutorial

## Run vLLM Load Testing Benchmark Job

To run a load testing benchmark job for vLLM, follow these steps:

### 1. Create and Run Benchmark Job

Create `vllm-benchmarking-job.yaml`:
```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: vllm-benchmark-job
spec:
  template:
    spec:
      containers:
      - name: vllm-benchmark
        image: quay.io/climatik-project/vllm-benchmark:latest
        command: ["/bin/sh"]
        args:
        - "-c"
        - "python3 benchmarks/benchmark_serving.py --backend vllm --host vllm-opt-125m --port 8000 --model facebook/opt-125m --dataset-name random --request-rate 100 --num-prompts 1000 --max-concurrency 50 --random-input-len 512 --random-output-len 128 --save-result --result-dir /results --result-filename benchmark_results.json"
        volumeMounts:
        - name: benchmark-results
          mountPath: /results
      volumes:
      - name: benchmark-results
        hostPath:
          path: /data/results
          type: Directory
      restartPolicy: Never
```

Run the benchmark job:
```bash
kubectl apply -f vllm-benchmarking-job.yaml
```

Monitor job progress:
```bash
kubectl get jobs
kubectl logs -l job-name=vllm-benchmark-job
```

Access results:
The benchmark results will be available in the `/data/results` directory on the host machine.

## Optional: Building Custom Benchmark Image

If you need to build a custom benchmark image (e.g., for deployment to specific registries), follow these steps:

1. From vLLM root directory, build the image:
```bash
# Using Docker
docker build --platform linux/amd64 -f Dockerfile.benchmark -t quay.io/climatik-project/vllm-benchmark:latest .

# Using Podman
podman build --platform linux/amd64 -f Dockerfile.benchmark -t quay.io/climatik-project/vllm-benchmark:latest .
```

2. Push to registry (optional):
```bash
# First login
podman login quay.io

# Then push
podman push quay.io/climatik-project/vllm-benchmark:latest
```

The image includes Python 3.9-slim, essential build tools, and vLLM testing requirements, using `benchmarks/benchmark_serving.py` as its entrypoint.

## Common Benchmark Parameters

The benchmark script supports several parameters:
- `--model`: HuggingFace model ID or local path
- `--tokenizer`: Tokenizer to use (defaults to model ID)
- `--test-throughput`: Enable throughput testing
- `--num-prompts`: Number of prompts to test (default: 1000)
- `--output-file`: Path to save results
- `--max-tokens`: Maximum number of tokens to generate (default: 100)
- `--tensor-parallel-size`: Number of GPUs for tensor parallelism

For a complete list of parameters:
```bash
kubectl run --rm -i --tty benchmark-help --image=quay.io/climatik-project/vllm-benchmark:latest -- --help
```

## Power Profiling with GPU Frequency Tuning

To analyze the relationship between GPU clock frequency and power consumption during benchmarking, you can use the provided frequency tuning script.

### 1. Prepare the Script

Create `run_benchmarks_with_freq.sh` in your benchmarks directory and make it executable:
```bash
chmod +x run_benchmarks_with_freq.sh
```

### 2. Run Power Profiling

The script will test the benchmark across different SM (Streaming Multiprocessor) clock frequencies:
```bash
./run_benchmarks_with_freq.sh
```

This script:
- Tests frequencies: 540MHz, 810MHz, 1110MHz, and 1410MHz
- Measures GPU power consumption during each benchmark run
- Collects detailed power metrics including average, standard deviation, and percentiles
- Saves both raw power data and summary statistics

### 3. Understanding the Results

Results are stored in `/data/results/` with several files:
- `power_summary.csv`: Overview of power statistics for each frequency
- `power_${freq}mhz_${timestamp}.log`: Detailed logs for each frequency run
- `power_${freq}mhz_${timestamp}_raw_power.csv`: Raw power measurements

Example power summary format:
```csv
frequency,average_power,std_dev,p95_power,p99_power
540,125.45,10.23,140.32,145.67
810,145.78,12.45,160.23,165.89
1110,180.34,15.67,
```

### 4. Cleanup

The script automatically:
- Cleans up benchmark jobs after each run
- Resets GPU frequency to default settings when complete
- Maintains a complete history of all runs

Note: This script requires:
- NVIDIA GPU with frequency tuning capabilities
- Administrative access to modify GPU settings
- Kubernetes cluster access
- Sufficient storage space for result collection

[Next tutorial section would continue here...] 
[Other tutorial sections would go here...]

## Optional: Building Custom Benchmark Image

If you need to build a custom benchmark image (e.g., for deployment to specific registries), follow these steps:

1. From vLLM root directory, build the image:
```bash
# Using Docker
docker build --platform linux/amd64 -f Dockerfile.benchmark -t quay.io/climatik-project/vllm-benchmark:latest .

# Using Podman
podman build --platform linux/amd64 -f Dockerfile.benchmark -t quay.io/climatik-project/vllm-benchmark:latest .
```

2. Push to registry (optional):
```bash
# First login
podman login quay.io

# Then push
podman push quay.io/climatik-project/vllm-benchmark:latest
```

The image includes Python 3.9-slim, essential build tools, and vLLM testing requirements, using `benchmarks/benchmark_serving.py` as its entrypoint.

[Next tutorial section would continue here...] 
 