package controllers

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/NVIDIA/go-nvml/pkg/nvml"

	v1alpha1 "github.com/Climatik-Project/Climatik-Project/freqtuner/api/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	cpufreqBasePath = "/sys/devices/system/cpu"
	scalingGovernor = "userspace"
)

// NodeFrequenciesReconciler reconciles a NodeFrequencies object
type NodeFrequenciesReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	NodeName string // Current node name where this controller is running
}

//+kubebuilder:rbac:groups=compute.example.com,resources=nodefrequencies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=compute.example.com,resources=nodefrequencies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=compute.example.com,resources=nodefrequencies/finalizers,verbs=update

// initNVML initializes the NVIDIA Management Library
func initNVML() error {
	return nvml.Init()
}

// shutdownNVML shuts down the NVIDIA Management Library
func shutdownNVML() error {
	return nvml.Shutdown()
}

// getGPUByUUID returns the NVIDIA device handle for a given UUID
func getGPUByUUID(uuid string) (nvml.Device, error) {
	return nvml.DeviceGetHandleByUUID(uuid)
}

func (r *NodeFrequenciesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Get the NodeFrequencies instance
	nodeFreq := &v1alpha1.NodeFrequencies{}
	if err := r.Get(ctx, req.NamespacedName, nodeFreq); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Only process if this CR belongs to this node
	if nodeFreq.Spec.NodeName != r.NodeName {
		return ctrl.Result{}, nil
	}

	// Update frequencies for GPUs
	if err := r.updateGPUFrequencies(ctx, nodeFreq); err != nil {
		log.Error(err, "failed to update GPU frequencies")
		return ctrl.Result{RequeueAfter: time.Second * 10}, err
	}

	// Update frequencies for CPUs
	if err := r.updateCPUFrequencies(ctx, nodeFreq); err != nil {
		log.Error(err, "failed to update CPU frequencies")
		return ctrl.Result{RequeueAfter: time.Second * 10}, err
	}

	// Update status
	if err := r.updateStatus(ctx, nodeFreq); err != nil {
		log.Error(err, "failed to update status")
		return ctrl.Result{RequeueAfter: time.Second * 5}, err
	}

	return ctrl.Result{RequeueAfter: time.Second * 30}, nil
}

func (r *NodeFrequenciesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.NodeFrequencies{}).
		Complete(r)
}

func (r *NodeFrequenciesReconciler) updateGPUFrequencies(ctx context.Context, nodeFreq *v1alpha1.NodeFrequencies) error {
	log := log.FromContext(ctx)

	// Initialize NVML
	if err := initNVML(); err != nil {
		return fmt.Errorf("failed to initialize NVML: %v", err)
	}
	defer shutdownNVML()

	// Process each GPU in parallel
	var wg sync.WaitGroup
	errChan := make(chan error, len(nodeFreq.Spec.GPUFrequencies))

	for _, gpuFreq := range nodeFreq.Spec.GPUFrequencies {
		wg.Add(1)
		go func(gpuSpec v1alpha1.GPUFrequencySpec) {
			defer wg.Done()

			// Get GPU handle
			device, err := getGPUByUUID(gpuSpec.UUID)
			if err != nil {
				errChan <- fmt.Errorf("failed to get GPU handle for %s: %v", gpuSpec.UUID, err)
				return
			}

			// Get current frequency
			currentGraphicsFreq, err := device.GetClockInfo(nvml.CLOCK_GRAPHICS)
			if err != nil {
				errChan <- fmt.Errorf("failed to get current frequency for GPU %s: %v", gpuSpec.UUID, err)
				return
			}

			// If graphics frequency needs updating
			if gpuSpec.GraphicsFrequency > 0 && uint32(currentGraphicsFreq) != uint32(gpuSpec.GraphicsFrequency) {
				err = device.SetApplicationsClocks(uint32(gpuSpec.MemoryFrequency), uint32(gpuSpec.GraphicsFrequency))
				if err != nil {
					errChan <- fmt.Errorf("failed to set frequency for GPU %s: %v", gpuSpec.UUID, err)
					return
				}

				log.Info("Updated GPU frequency",
					"gpuUUID", gpuSpec.UUID,
					"oldFreq", currentGraphicsFreq,
					"newGraphicsFreq", gpuSpec.GraphicsFrequency)
			}
		}(gpuFreq)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Collect any errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("encountered errors updating GPU frequencies: %v", errors)
	}

	return nil
}

// findClosestFrequency finds the closest supported frequency to the target
func findClosestFrequency(supported []uint32, target uint32) uint32 {
	if len(supported) == 0 {
		return target
	}

	closest := supported[0]
	minDiff := uint32(^uint32(0)) // Maximum uint32 value

	for _, freq := range supported {
		diff := uint32(0)
		if freq > target {
			diff = freq - target
		} else {
			diff = target - freq
		}

		if diff < minDiff {
			minDiff = diff
			closest = freq
		}
	}

	return closest
}

func (r *NodeFrequenciesReconciler) updateCPUFrequencies(ctx context.Context, nodeFreq *v1alpha1.NodeFrequencies) error {
	log := log.FromContext(ctx)

	var wg sync.WaitGroup
	errChan := make(chan error, len(nodeFreq.Spec.CPUFrequencies))

	for _, cpuFreq := range nodeFreq.Spec.CPUFrequencies {
		wg.Add(1)
		go func(cpuSpec v1alpha1.CPUFrequencySpec) {
			defer wg.Done()

			cpuPath := filepath.Join(cpufreqBasePath, fmt.Sprintf("cpu%d/cpufreq", cpuSpec.CoreID))

			// 1. Set the scaling governor to userspace
			governorPath := filepath.Join(cpuPath, "scaling_governor")
			if err := ioutil.WriteFile(governorPath, []byte(scalingGovernor), 0644); err != nil {
				errChan <- fmt.Errorf("failed to set governor for CPU %d: %v", cpuSpec.CoreID, err)
				return
			}

			// 2. Get current frequency
			currentFreqPath := filepath.Join(cpuPath, "scaling_cur_freq")
			freqBytes, err := ioutil.ReadFile(currentFreqPath)
			if err != nil {
				errChan <- fmt.Errorf("failed to read current frequency for CPU %d: %v", cpuSpec.CoreID, err)
				return
			}

			currentFreqKHz, err := strconv.ParseInt(string(freqBytes[:len(freqBytes)-1]), 10, 64)
			if err != nil {
				errChan <- fmt.Errorf("failed to parse current frequency for CPU %d: %v", cpuSpec.CoreID, err)
				return
			}
			currentFreqMHz := int32(currentFreqKHz / 1000)

			// 3. Check if frequency update is needed
			if currentFreqMHz != cpuSpec.Frequency {
				// Find closest supported frequency
				targetFreqKHz := cpuSpec.Frequency * 1000
				setspeedPath := filepath.Join(cpuPath, "scaling_setspeed")

				// Set the new frequency
				if err := os.WriteFile(setspeedPath, []byte(strconv.FormatInt(int64(targetFreqKHz), 10)), 0644); err != nil {
					errChan <- fmt.Errorf("failed to set frequency for CPU %d: %v", cpuSpec.CoreID, err)
					return
				}

				// Verify the change
				newFreqBytes, err := ioutil.ReadFile(currentFreqPath)
				if err != nil {
					errChan <- fmt.Errorf("failed to verify new frequency for CPU %d: %v", cpuSpec.CoreID, err)
					return
				}

				newFreqKHz, err := strconv.ParseInt(string(newFreqBytes[:len(newFreqBytes)-1]), 10, 64)
				if err != nil {
					errChan <- fmt.Errorf("failed to parse new frequency for CPU %d: %v", cpuSpec.CoreID, err)
					return
				}

				log.Info("Updated CPU frequency",
					"cpuId", cpuSpec.CoreID,
					"oldFreq", currentFreqMHz,
					"targetFreq", cpuSpec.Frequency,
					"actualFreq", newFreqKHz/1000)
			}
		}(cpuFreq)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errChan)

	// Collect any errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("encountered errors updating CPU frequencies: %v", errors)
	}

	return nil
}

// InitializeNodeFrequenciesCRs creates a NodeFrequencies CR for the current node if it doesn't exist
func (r *NodeFrequenciesReconciler) InitializeNodeFrequenciesCRs(ctx context.Context) error {
	log := ctrl.Log.WithName("initialize-node-frequencies")

	// Only create CR for the current node
	nodeFreq := &v1alpha1.NodeFrequencies{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Name:      r.NodeName,
		Namespace: "default",
	}, nodeFreq)
	if err != nil && errors.IsNotFound(err) {
		// Create a new NodeFrequencies CR
		newNodeFreq := &v1alpha1.NodeFrequencies{
			ObjectMeta: metav1.ObjectMeta{
				Name:      r.NodeName,
				Namespace: "default",
			},
			Spec: v1alpha1.NodeFrequenciesSpec{
				NodeName: r.NodeName,
			},
		}

		if err := r.Client.Create(ctx, newNodeFreq); err != nil {
			return fmt.Errorf("failed to create NodeFrequencies CR for node %s: %v", r.NodeName, err)
		}

		log.Info("Created NodeFrequencies CR", "nodeName", r.NodeName)
	} else if err != nil {
		return fmt.Errorf("failed to get NodeFrequencies CR for node %s: %v", r.NodeName, err)
	}

	return nil
}

func (r *NodeFrequenciesReconciler) updateStatus(ctx context.Context, nodeFreq *v1alpha1.NodeFrequencies) error {
	return r.Status().Update(ctx, nodeFreq)
}
