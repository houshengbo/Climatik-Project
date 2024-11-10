#!/bin/bash

# Get the directory where the script is located
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

while true; do
    echo "=== Starting new benchmark run at $(date) ==="
    
    # Use SCRIPT_DIR to reference the yaml file
    kubectl apply -f "${SCRIPT_DIR}/vllm-benchmarking-job.yaml"
    
    # Wait for job to complete
    while true; do
        status=$(kubectl get job vllm-benchmark-job -o jsonpath='{.status.conditions[?(@.type=="Complete")].status}')
        if [ "$status" == "True" ]; then
            echo "Job completed successfully at $(date)"
            break
        fi
        
        failed=$(kubectl get job vllm-benchmark-job -o jsonpath='{.status.failed}')
        if [ "$failed" != "" ] && [ "$failed" != "0" ]; then
            echo "Job failed at $(date)"
            break
        fi
        
        echo "Job still running... waiting 10 seconds"
        sleep 10
    done
    
    # Clean up
    echo "Cleaning up job and pod..."
    kubectl delete job vllm-benchmark-job --wait=false
    
    echo "Waiting 5 seconds before starting next run..."
    sleep 5
done