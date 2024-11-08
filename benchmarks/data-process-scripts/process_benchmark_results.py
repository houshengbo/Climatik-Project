import sys
import json
import numpy as np

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

def process_benchmark_results(result_file, perf_summary, freq):
    """Process benchmark results and append summary statistics to the summary file."""
    # Load benchmark results
    with open(result_file, 'r') as f:
        data = json.load(f)
    
    # Extract available metrics
    request_throughput = data['request_throughput']
    output_throughput = data['output_throughput']
    total_token_throughput = data['total_token_throughput']
    
    # Calculate statistics for input lengths
    input_len_stats = calculate_stats(data['input_lens'])
    
    # Write to performance summary
    with open(perf_summary, 'a') as f:
        # Write input length statistics
        f.write(f'{freq},{input_len_stats[0]:.2f},{input_len_stats[1]:.2f},')
        f.write(f'{input_len_stats[2]:.2f},{input_len_stats[3]:.2f},')
        
        # Write throughput metrics
        f.write(f'{request_throughput:.2f},{output_throughput:.2f},')
        f.write(f'{total_token_throughput:.2f}\n')

if __name__ == '__main__':
    if len(sys.argv) != 4:
        print("Usage: python process_benchmark_results.py <result_file> <perf_summary> <frequency>")
        sys.exit(1)
        
    result_file = sys.argv[1]
    perf_summary = sys.argv[2]
    freq = int(sys.argv[3])
    
    process_benchmark_results(result_file, perf_summary, freq) 