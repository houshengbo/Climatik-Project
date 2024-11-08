#!/bin/bash

# Add this at the beginning of the script if not already present
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
RESULTS_DIR="${RESULTS_DIR:-"/data/results"}"  # Default to /data/results if not set

# Create results directory if it doesn't exist
mkdir -p "$RESULTS_DIR"

# Output summary files
POWER_SUMMARY="$RESULTS_DIR/power_summary.csv"
PERF_SUMMARY="$RESULTS_DIR/performance_summary.csv"

# Initialize summary files with headers
echo "frequency,average_power,std_dev,p95_power,p99_power" > "$POWER_SUMMARY"
echo "frequency,mean_ttft_ms,median_ttft_ms,std_ttft_ms,p99_ttft_ms,mean_tpot_ms,median_tpot_ms,std_tpot_ms,p99_tpot_ms,mean_itl_ms,median_itl_ms,std_itl_ms,p99_itl_ms,request_throughput,output_throughput,total_token_throughput" > "$PERF_SUMMARY"

# Setup Python virtual environment
VENV_DIR="$HOME/.venv/benchmark_venv"
if [ ! -d "$VENV_DIR" ]; then
    echo "Creating new virtual environment at $VENV_DIR"
    python3 -m venv "$VENV_DIR"
else
    echo "Using existing virtual environment at $VENV_DIR"
fi
source "$VENV_DIR/bin/activate"

# Install requirements
pip install -r "$SCRIPT_DIR/data-process-scripts/requirements.txt"

# Function to calculate statistics from benchmark results
process_benchmark_results() {
    local freq=$1
    local result_file="$RESULTS_DIR/benchmark_results_${freq}mhz.json"
    
    python3 "$SCRIPT_DIR/data-process-scripts/process_benchmark_results.py" \
        "$result_file" \
        "$PERF_SUMMARY" \
        "$freq"
}

# Set default memory and graphics clock if not already set
MEM_CLOCK=${MEM_CLOCK:-"877"}
MAX_CLOCK=${MAX_CLOCK:-"1380"}

# Get supported clock frequencies dynamically
FREQUENCIES=($(nvidia-smi -q -d SUPPORTED_CLOCKS | grep "Graphics" | sed 's/.*: //; s/\s\+/ /g; s/MHz//g' | tr ' ' '\n' | sort -nr | uniq))

# If no frequencies found, exit with error
if [ ${#FREQUENCIES[@]} -eq 0 ]; then
    echo "Error: Could not get supported GPU frequencies"
    exit 1
fi

echo "Testing the following frequencies (MHz):"
printf '%s\n' "${FREQUENCIES[@]}"

for freq in "${FREQUENCIES[@]}"; do
    echo "Testing frequency: ${freq}MHz"
    
    # Get current timestamp once for both files
    TIMESTAMP=$(date +%Y%m%d_%H%M%S)
    POWER_FILE="$RESULTS_DIR/power_${freq}mhz_${TIMESTAMP}_raw_power.csv"
    
    # Create a temporary job yaml with frequency-specific names
    TMP_JOB_YAML="/tmp/vllm-benchmark-job-${freq}.yaml"
    sed "s/name: vllm-benchmark-job/name: vllm-benchmark-job-${freq}/" "$SCRIPT_DIR/vllm-benchmarking-job.yaml" | \
    sed "s/benchmark_results.json/benchmark_results_${freq}mhz.json/" > "$TMP_JOB_YAML"
    
    # Set GPU frequency using MEM_CLOCK variable
    sudo nvidia-smi -ac ${MEM_CLOCK},${freq}
    
    # Start power monitoring in background
    nvidia-smi --query-gpu=timestamp,power.draw --format=csv,nounits -l 1 > "$POWER_FILE" &
    POWER_PID=$!
    
    # Run benchmark with temporary yaml
    if ! kubectl apply -f "$TMP_JOB_YAML"; then
        echo "Error: Failed to create kubernetes job"
        kill $POWER_PID
        rm "$TMP_JOB_YAML"
        continue
    fi
    
    # Wait for job completion with correct job name
    if ! kubectl wait --for=condition=complete "job/vllm-benchmark-job-${freq}" --timeout=1h; then
        echo "Error: Kubernetes job failed or timed out"
        kill $POWER_PID
        kubectl delete -f "$TMP_JOB_YAML"
        rm "$TMP_JOB_YAML"
        continue
    fi
    
    # Stop power monitoring
    kill $POWER_PID
    
    # Process power data with correct path
    python3 "$SCRIPT_DIR/data-process-scripts/process_power_data.py" \
        "$POWER_FILE" \
        "$POWER_SUMMARY" \
        "$freq"
    
    # Process benchmark results
    process_benchmark_results $freq
    
    # Cleanup
    kubectl delete job vllm-benchmark-job-${freq}
    sleep 30  # Wait for resources to clean up
done

# Reset GPU frequency to default using MEM_CLOCK and MAX_CLOCK variables
sudo nvidia-smi -ac ${MEM_CLOCK},${MAX_CLOCK}

# Deactivate virtual environment
deactivate

# Clean up virtual environment
rm -rf "$VENV_DIR"

echo "Benchmark complete. Results available in $RESULTS_DIR"
echo "Performance summary: $PERF_SUMMARY"
echo "Power summary: $POWER_SUMMARY"