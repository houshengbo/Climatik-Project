#!/bin/bash

# Define the SM clock frequencies to test (in MHz)
declare -a sm_frequencies=(540 810 1110 1410)

# Function to get average GPU power over a period
measure_gpu_power() {
    local output_file=$1
    local duration=$2  # in seconds
    local interval=1   # sample every second
    local samples=0
    local total_power=0

    echo "Starting power measurement..."
    
    # Create raw power data file
    local raw_power_file="${output_file%.*}_raw_power.csv"
    echo "timestamp,power_watts" > "$raw_power_file"
    
    # Sample power usage every second
    for ((i=0; i<duration; i++)); do
        local timestamp=$(date +"%Y-%m-%d %H:%M:%S")
        power=$(nvidia-smi --query-gpu=power.draw --format=csv,noheader,nounits)
        
        # Save raw data with timestamp
        echo "$timestamp,$power" >> "$raw_power_file"
        
        total_power=$(echo "$total_power + $power" | bc)
        samples=$((samples + 1))
        sleep $interval
    done

    # Calculate average
    average_power=$(echo "scale=2; $total_power / $samples" | bc)
    echo "Average GPU Power: ${average_power}W" | tee -a "$output_file"
    echo "Raw power data saved to: $raw_power_file" | tee -a "$output_file"
}

# Function to calculate percentile from sorted data
get_percentile() {
    local sorted_file=$1
    local percentile=$2
    local line_count=$(wc -l < "$sorted_file")
    local line_number=$(echo "scale=0; $line_count * $percentile / 100" | bc)
    sed -n "${line_number}p" "$sorted_file" | cut -d',' -f2
}

# Function to calculate statistics from raw power data
calculate_power_stats() {
    local raw_power_file=$1
    local freq=$2
    local summary_file="/data/results/power_summary.csv"
    
    # Create summary file with headers if it doesn't exist
    if [ ! -f "$summary_file" ]; then
        echo "frequency,average_power,std_dev,p95_power,p99_power" > "$summary_file"
    }
    
    # Calculate statistics using awk and sort
    # Skip header line, take only power values
    local stats=$(tail -n +2 "$raw_power_file" | cut -d',' -f2 | awk '
        BEGIN {
            sum = 0
            sum_sq = 0
            count = 0
        }
        {
            sum += $1
            sum_sq += $1 * $1
            values[count++] = $1
        }
        END {
            avg = sum / count
            std = sqrt(sum_sq/count - (sum/count)^2)
            printf "%.2f,%.2f", avg, std
        }
    ')
    
    # Calculate percentiles
    # Create sorted file for percentile calculation
    local sorted_file="/tmp/sorted_power.txt"
    tail -n +2 "$raw_power_file" | sort -t',' -k2 -n > "$sorted_file"
    
    local p95=$(get_percentile "$sorted_file" 95)
    local p99=$(get_percentile "$sorted_file" 99)
    
    # Add to summary file
    echo "${freq},${stats},${p95},${p99}" >> "$summary_file"
    
    # Clean up temporary file
    rm "$sorted_file"
    
    echo "Added power statistics for ${freq}MHz to $summary_file"
}

# Function to run benchmark for a specific frequency
run_benchmark_at_freq() {
    local freq=$1
    
    echo "Setting GPU Clock to ${freq} MHz"
    # Set GPU frequency (assuming GPU 0, modify -i if needed)
    nvidia-smi -i 0 -ac 1215,$freq
    if [ $? -ne 0 ]; then
        echo "Failed to set GPU Clock to ${freq} MHz"
        return 1
    fi
    
    # Wait for frequency to stabilize
    sleep 10
    
    # Create results directory if it doesn't exist
    mkdir -p /data/results
    
    # Create a timestamp for this run
    timestamp=$(date +%Y%m%d_%H%M%S)
    power_log="/data/results/power_${freq}mhz_${timestamp}.log"
    
    echo "Starting benchmark at ${freq} MHz at $(date)" | tee "$power_log"
    
    # Start the benchmark job with frequency in the name
    cat manifests/vllm-benchmarking-job.yaml | \
        sed "s/benchmark_results.json/benchmark_${freq}mhz.json/g" | \
        sed "s/vllm-benchmark-job/vllm-benchmark-job-${freq}mhz/g" | \
        kubectl apply -f -
    
    # Wait for job to start
    echo "Waiting for job to start..."
    kubectl wait --for=condition=ready pod -l job-name=vllm-benchmark-job-${freq}mhz --timeout=60s
    
    # Measure power while benchmark is running
    # Assuming benchmark takes about 5 minutes (300 seconds)
    measure_gpu_power "$power_log" 300
    
    # Wait for job to complete
    echo "Waiting for job to complete..."
    kubectl wait --for=condition=complete job/vllm-benchmark-job-${freq}mhz --timeout=600s
    
    # Get job logs
    kubectl logs job/vllm-benchmark-job-${freq}mhz >> "$power_log"
    
    # Clean up the job
    kubectl delete job vllm-benchmark-job-${freq}mhz
    
    # After benchmark completes and power data is collected
    calculate_power_stats "${power_log%.*}_raw_power.csv" "$freq"
}

# Main script
echo "Starting GPU frequency benchmarking suite"
echo "----------------------------------------"

# Clear previous summary file if it exists
rm -f /data/results/power_summary.csv

# Run benchmarks for each frequency
for freq in "${sm_frequencies[@]}"; do
    run_benchmark_at_freq $freq
    sleep 30
done

# Print final summary
echo "----------------------------------------"
echo "Benchmarking complete. Results summary:"
echo "----------------------------------------"
cat /data/results/power_summary.csv
echo "----------------------------------------"
echo "Detailed results are in /data/results/"

# Reset GPU frequency to default
nvidia-smi -rgc