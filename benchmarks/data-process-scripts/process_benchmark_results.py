import sys
import json
import numpy as np
import os

def calculate_stats(data):
    """Calculate statistical metrics for a data series.
    
    Args:
        data (list): List of numerical values
    
    Returns:
        tuple: (mean, median, standard deviation, 99th percentile)
    """
    mean = np.mean(data)
    median = np.median(data)
    std = np.std(data)
    p99 = np.percentile(data, 99)
    return mean, median, std, p99

def extract_metrics(data):
    """Extract throughput and latency metrics from benchmark results.
    
    Args:
        data (dict): Benchmark results JSON data
    
    Returns:
        dict: Dictionary containing performance metrics
    """
    return {
        key: value for key, value in data.items() 
        if 'throughput' in key or key.endswith('ms')
    }

def process_benchmark_results(result_file, perf_summary, freq):
    """Process benchmark results and append summary statistics to the summary file."""
    # Load benchmark results
    with open(result_file, 'r') as f:
        data = json.load(f)
    
    # Extract available metrics
    metrics = extract_metrics(data)
    
    # Check if file exists and is empty
    is_new_file = not os.path.exists(perf_summary) or os.path.getsize(perf_summary) == 0
    
    with open(perf_summary, 'a') as f:
        # Write headers if file is new/empty
        if is_new_file:
            headers = ['frequency'] + list(metrics.keys())
            f.write(','.join(headers) + '\n')
        
        # Write metrics
        values = [str(freq)] + [f'{value:.2f}' for value in metrics.values()]
        f.write(','.join(values) + '\n')

if __name__ == '__main__':
    if len(sys.argv) != 4:
        print("Usage: python process_benchmark_results.py <result_file> <perf_summary> <frequency>")
        sys.exit(1)
        
    result_file = sys.argv[1]
    perf_summary = sys.argv[2]
    freq = int(sys.argv[3])
    
    process_benchmark_results(result_file, perf_summary, freq) 