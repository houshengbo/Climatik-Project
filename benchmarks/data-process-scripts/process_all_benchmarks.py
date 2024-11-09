#!/usr/bin/env python3
import os
import sys
import glob
from process_benchmark_results import process_benchmark_results

def process_all_results(data_folder):
    """Process all benchmark result files in the given folder.
    
    Args:
        data_folder (str): Path to folder containing benchmark results
    """
    # Set output file in the same data folder
    output_file = os.path.join(data_folder, 'performance_summary.csv')
    print(f"Will write results to: {output_file}")
    
    # Find all benchmark result files
    pattern = os.path.join(data_folder, 'benchmark_results_*.json')
    result_files = glob.glob(pattern)
    
    if not result_files:
        print(f"No benchmark result files found in {data_folder}")
        return
    
    # Process each file
    def get_freq(filename):
        return int(filename.split('_')[-1].replace('.json', '').replace('mhz', ''))
    
    # Sort files by frequency in descending order
    sorted_files = sorted(result_files, key=get_freq, reverse=True)
    
    for result_file in sorted_files:
        freq = get_freq(result_file)
        print(f"Processing results for {freq}MHz...")
        
        # Process the benchmark results
        process_benchmark_results(result_file, output_file, freq)
    
    print(f"Successfully processed {len(result_files)} files")
    print(f"Results written to: {output_file}")

if __name__ == '__main__':
    if len(sys.argv) != 2:
        print("Usage: python process_all_benchmarks.py <data_folder>")
        sys.exit(1)
    
    data_folder = sys.argv[1]
    if not os.path.isdir(data_folder):
        print(f"Error: {data_folder} is not a directory")
        sys.exit(1)
        
    process_all_results(data_folder) 