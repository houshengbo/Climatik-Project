import sys
import pandas as pd

def process_power_data(power_file, power_summary, freq):
    """Process power monitoring data and append summary statistics to the summary file.
    
    Args:
        power_file (str): Path to the raw power monitoring CSV file
        power_summary (str): Path to the power summary CSV file
        freq (int): GPU frequency in MHz
    """
    # Read power data, skip header row
    power_data = pd.read_csv(power_file, skiprows=1)
    power_values = power_data.iloc[:, 1]  # Second column contains power values
    
    # Calculate statistics
    stats = {
        'mean': power_values.mean(),
        'std': power_values.std(),
        'p95': power_values.quantile(0.95),
        'p99': power_values.quantile(0.99)
    }
    
    # Append to summary file
    with open(power_summary, 'a') as f:
        f.write(f'{freq},{stats["mean"]:.2f},{stats["std"]:.2f},')
        f.write(f'{stats["p95"]:.2f},{stats["p99"]:.2f}\n')

if __name__ == '__main__':
    if len(sys.argv) != 4:
        print("Usage: python process_power_data.py <power_file> <power_summary> <frequency>")
        sys.exit(1)
        
    power_file = sys.argv[1]
    power_summary = sys.argv[2]
    freq = int(sys.argv[3])
    
    process_power_data(power_file, power_summary, freq) 