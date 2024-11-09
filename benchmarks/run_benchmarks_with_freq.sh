#!/bin/bash

# Function to show usage
show_usage() {
    echo "Usage: $0 [-h] [job_yaml_path]"
    echo
    echo "Run benchmarks with different GPU frequencies"
    echo
    echo "Options:"
    echo "  -h              Show this help message"
    echo "  job_yaml_path   Path to the job YAML file"
    echo "                  (default: ./vllm-benchmarking-job.yaml)"
    exit 1
}

# Parse command line options
while getopts "h" opt; do
    case ${opt} in
        h )
            show_usage
            ;;
        \? )
            show_usage
            ;;
    esac
done
shift $((OPTIND -1))

# Set job YAML path
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
BASE_JOB_YAML="${1:-"$SCRIPT_DIR/vllm-benchmarking-job.yaml"}"

# Verify the job yaml exists
if [ ! -f "$BASE_JOB_YAML" ]; then
    echo "Error: Job YAML file not found: $BASE_JOB_YAML"
    exit 1
fi

# Add this at the beginning of the script if not already present
RESULTS_DIR="${RESULTS_DIR:-"/data/results"}"  # Default to /data/results if not set

# Create results directory if it doesn't exist
mkdir -p "$RESULTS_DIR"

# Output summary files
POWER_SUMMARY="$RESULTS_DIR/power_summary.csv"
PERF_SUMMARY="$RESULTS_DIR/performance_summary.csv"

# Initialize summary files with headers
echo "frequency,average_power,std_dev,p95_power,p99_power" > "$POWER_SUMMARY"

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
    sed "s/name: vllm-benchmark-job/name: vllm-benchmark-job-${freq}/" "$BASE_JOB_YAML" | \
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
    python3 "$SCRIPT_DIR/data-process-scripts/process_benchmark_results.py" \
        "$RESULTS_DIR/benchmark_results_${freq}mhz.json" \
        "$PERF_SUMMARY" \
        "$freq"
    
    # Cleanup
    kubectl delete job vllm-benchmark-job-${freq}
    sleep 30  # Wait for resources to clean up
done

# Reset GPU frequency to default using MEM_CLOCK and MAX_CLOCK variables
sudo nvidia-smi -ac ${MEM_CLOCK},${MAX_CLOCK}

# Deactivate virtual environment
deactivate

echo "Benchmark complete. Results available in $RESULTS_DIR"
echo "Performance summary: $PERF_SUMMARY"
echo "Power summary: $POWER_SUMMARY"