package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/NVIDIA/go-nvml/pkg/nvml"

	v1alpha1 "github.com/Climatik-Project/Climatik-Project/freqtuner/api/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	cpufreqBasePath = "/sys/devices/system/cpu"
	scalingGovernor = "userspace"
)

// NodeFrequenciesReconciler reconciles a NodeFrequencies object
type NodeFrequenciesReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	NodeName        string              // Current node name where this controller is running
	Namespace       string              // Namespace for CRs
	nvmlInitialized bool                // Track NVML initialization state
	nvmlMutex       sync.Mutex          // Protect NVML initialization
	gpuFreqCache    map[string][]uint32 // Map of GPU UUID to supported frequencies
	cpuFreqCache    map[int32][]uint32  // Map of CPU ID to supported frequencies
	cacheMutex      sync.RWMutex
}

//+kubebuilder:rbac:groups=compute.example.com,resources=nodefrequencies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=compute.example.com,resources=nodefrequencies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=compute.example.com,resources=nodefrequencies/finalizers,verbs=update

// getGPUByUUID returns the NVIDIA device handle for a given UUID
func getGPUByUUID(uuid string) (nvml.Device, error) {
	device, ret := nvml.DeviceGetHandleByUUID(uuid)
	if ret != nvml.SUCCESS {
		return device, fmt.Errorf("NVML error getting device handle: %v", ret)
	}
	return device, nil
}

func (r *NodeFrequenciesReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	log.Info("Starting reconciliation", "request", req)

	log.Info("About to get NodeFrequencies instance",
		"namespace", req.Namespace,
		"name", req.Name)

	// Get the NodeFrequencies instance
	nodeFreq := &v1alpha1.NodeFrequencies{}
	err := r.Get(ctx, req.NamespacedName, nodeFreq)
	if err != nil {
		log.Error(err, "Unable to fetch NodeFrequencies")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("After getting NodeFrequencies instance",
		"error", err,
		"nodeFreq", nodeFreq)

	log.Info("Processing NodeFrequencies",
		"name", nodeFreq.Name,
		"gpuCount", len(nodeFreq.Spec.GPUFrequencies))

	// Only process if this CR belongs to this node
	if nodeFreq.Spec.NodeName != r.NodeName {
		log.Info("Skipping reconciliation for different node",
			"CR.NodeName", nodeFreq.Spec.NodeName,
			"Controller.NodeName", r.NodeName)
		return ctrl.Result{}, nil
	}

	// Only update GPU frequencies if spec and status differ
	if !reflect.DeepEqual(nodeFreq.Spec.GPUFrequencies, nodeFreq.Status.GPUFrequencies) {
		log.Info("GPU frequencies need update",
			"spec", nodeFreq.Spec.GPUFrequencies,
			"status", nodeFreq.Status.GPUFrequencies)
		if err := r.updateGPUFrequencies(ctx, nodeFreq); err != nil {
			log.Error(err, "failed to update GPU frequencies")
			return ctrl.Result{RequeueAfter: time.Second * 10}, err
		}
	}

	// Only update CPU frequencies if spec and status differ
	if !reflect.DeepEqual(nodeFreq.Spec.CPUFrequencies, nodeFreq.Status.CPUFrequencies) {
		log.Info("CPU frequencies need update",
			"spec", nodeFreq.Spec.CPUFrequencies,
			"status", nodeFreq.Status.CPUFrequencies)
		if err := r.updateCPUFrequencies(ctx, nodeFreq); err != nil {
			log.Error(err, "failed to update CPU frequencies")
			return ctrl.Result{RequeueAfter: time.Second * 10}, err
		}
	}

	// Update status
	if err := r.updateStatus(ctx, nodeFreq); err != nil {
		log.Error(err, "failed to update status")
		return ctrl.Result{RequeueAfter: time.Second * 5}, err
	}

	return ctrl.Result{RequeueAfter: time.Second * 30}, nil
}

func (r *NodeFrequenciesReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Initialize caches
	r.gpuFreqCache = make(map[string][]uint32)
	r.cpuFreqCache = make(map[int32][]uint32)

	// Initialize NVML and frequency caches
	if err := r.InitializeNVML(); err != nil {
		return fmt.Errorf("failed to initialize NVML: %v", err)
	}
	if err := r.initializeFrequencyCaches(); err != nil {
		return fmt.Errorf("failed to initialize frequency caches: %v", err)
	}

	// Ensure NVML is shutdown when controller stops
	mgr.Add(manager.RunnableFunc(func(context.Context) error {
		r.ShutdownNVML()
		return nil
	}))

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.NodeFrequencies{}).
		Complete(r)
}

// InitializeNVML safely initializes NVML once
func (r *NodeFrequenciesReconciler) InitializeNVML() error {
	r.nvmlMutex.Lock()
	defer r.nvmlMutex.Unlock()

	if !r.nvmlInitialized {
		ret := nvml.Init()
		if ret != nvml.SUCCESS {
			return fmt.Errorf("NVML initialization failed: %v", ret)
		}
		r.nvmlInitialized = true
	}
	return nil
}

// ShutdownNVML safely shuts down NVML
func (r *NodeFrequenciesReconciler) ShutdownNVML() {
	r.nvmlMutex.Lock()
	defer r.nvmlMutex.Unlock()

	log := ctrl.Log.WithName("nvml-shutdown")
	log.Info("Shutting down NVML", "currentState", r.nvmlInitialized)

	if r.nvmlInitialized {
		nvml.Shutdown()
		r.nvmlInitialized = false
		log.Info("NVML shutdown complete")
	}
}

// Add this method to safely check NVML state
func (r *NodeFrequenciesReconciler) isNVMLInitialized() bool {
	r.nvmlMutex.Lock()
	defer r.nvmlMutex.Unlock()
	return r.nvmlInitialized
}

func (r *NodeFrequenciesReconciler) updateGPUFrequencies(ctx context.Context, nodeFreq *v1alpha1.NodeFrequencies) error {
	log := log.FromContext(ctx)

	// Add state logging
	log.Info("Checking NVML state before GPU updates",
		"nvmlInitialized", r.isNVMLInitialized())

	// Add verification of NVML initialization
	if !r.isNVMLInitialized() {
		log.Error(fmt.Errorf("NVML not initialized"),
			"attempting re-initialization")
		if err := r.InitializeNVML(); err != nil {
			return fmt.Errorf("failed to re-initialize NVML: %v", err)
		}
	}

	// Get GPU count with better error handling
	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		log.Error(fmt.Errorf("NVML error: %v", ret), "Failed to get GPU count",
			"errorCode", ret,
			"nvmlInitialized", r.nvmlInitialized)
		return fmt.Errorf("failed to get GPU count: %v", ret)
	}

	log.Info("Successfully retrieved GPU count",
		"count", count,
		"nvmlInitialized", r.nvmlInitialized)

	// If no GPUs in spec but GPUs exist, initialize the spec
	if len(nodeFreq.Spec.GPUFrequencies) == 0 {
		for i := 0; i < count; i++ {
			device, ret := nvml.DeviceGetHandleByIndex(i)
			if ret != nvml.SUCCESS {
				continue
			}

			uuid, ret := device.GetUUID()
			if ret != nvml.SUCCESS {
				continue
			}

			nodeFreq.Spec.GPUFrequencies = append(nodeFreq.Spec.GPUFrequencies, v1alpha1.GPUFrequencySpec{
				UUID: uuid,
			})
		}
		// Update the CR with discovered GPUs
		if err := r.Update(ctx, nodeFreq); err != nil {
			log.Error(err, "Failed to update NodeFrequencies with GPU information")
			return err
		}
	}

	// Process GPUs sequentially
	for _, gpuSpec := range nodeFreq.Spec.GPUFrequencies {
		log.Info("Processing GPU", "UUID", gpuSpec.UUID)

		// Get GPU handle
		device, err := getGPUByUUID(gpuSpec.UUID)
		if err != nil {
			log.Error(err, "Failed to get GPU handle", "UUID", gpuSpec.UUID)
			continue
		}

		// Get current frequency
		currentGraphicsFreq, ret := device.GetClockInfo(nvml.CLOCK_GRAPHICS)
		if ret != nvml.SUCCESS {
			log.Error(fmt.Errorf("failed to get current frequency: %v", ret),
				"UUID", gpuSpec.UUID)
			continue
		}

		// If graphics frequency needs updating
		if gpuSpec.GraphicsFrequency > 0 && uint32(currentGraphicsFreq) != uint32(gpuSpec.GraphicsFrequency) {
			// Log initial state
			currentSMClock, _ := device.GetClockInfo(nvml.CLOCK_SM)
			currentMemClock, _ := device.GetClockInfo(nvml.CLOCK_MEM)

			log.Info("Current GPU state",
				"UUID", gpuSpec.UUID,
				"graphicsClock", currentGraphicsFreq,
				"smClock", currentSMClock,
				"memClock", currentMemClock)

			// Get supported clocks for this GPU
			r.cacheMutex.RLock()
			supportedFreqs := r.gpuFreqCache[gpuSpec.UUID]
			r.cacheMutex.RUnlock()

			targetFreq := uint32(gpuSpec.GraphicsFrequency)
			closestFreq := findClosestFrequency(supportedFreqs, targetFreq)

			// Set application clocks first
			ret = device.SetApplicationsClocks(uint32(gpuSpec.MemoryFrequency), closestFreq)
			if ret != nvml.SUCCESS {
				log.Error(fmt.Errorf("failed to set application clocks: %v", ret),
					"UUID", gpuSpec.UUID,
					"targetFreq", closestFreq,
					"memFreq", gpuSpec.MemoryFrequency)
				continue
			}

			// Force GPU clocks by setting constraints
			ret = device.SetGpuLockedClocks(closestFreq, closestFreq)
			if ret != nvml.SUCCESS {
				log.Error(fmt.Errorf("failed to set GPU locked clocks: %v", ret),
					"UUID", gpuSpec.UUID,
					"targetFreq", closestFreq)
			}

			// Add persistence mode to prevent clock reset
			ret = device.SetPersistenceMode(nvml.FEATURE_ENABLED)
			if ret != nvml.SUCCESS {
				log.Error(fmt.Errorf("failed to set persistence mode: %v", ret),
					"UUID", gpuSpec.UUID)
			}

			// Verify the changes
			time.Sleep(time.Second)
			newGraphicsClock, _ := device.GetClockInfo(nvml.CLOCK_GRAPHICS)
			newSMClock, _ := device.GetClockInfo(nvml.CLOCK_SM)
			newMemClock, _ := device.GetClockInfo(nvml.CLOCK_MEM)

			log.Info("GPU clocks after update",
				"UUID", gpuSpec.UUID,
				"targetFreq", closestFreq,
				"currentGraphicsClock", newGraphicsClock,
				"currentSMClock", newSMClock,
				"currentMemClock", newMemClock)
		}
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
				// Get available frequencies
				r.cacheMutex.RLock()
				availableFreqs := r.cpuFreqCache[cpuSpec.CoreID]
				r.cacheMutex.RUnlock()

				// Find closest supported frequency
				targetFreqKHz := uint32(cpuSpec.Frequency * 1000)
				closestFreq := findClosestFrequency(availableFreqs, targetFreqKHz)

				// Set the new frequency
				setspeedPath := filepath.Join(cpuPath, "scaling_setspeed")
				if err := os.WriteFile(setspeedPath, []byte(strconv.FormatInt(int64(closestFreq), 10)), 0644); err != nil {
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
		Namespace: r.Namespace,
	}, nodeFreq)

	if err != nil && errors.IsNotFound(err) {
		// Check NVML initialization state
		log.Info("Checking NVML state before initialization",
			"nvmlInitialized", r.isNVMLInitialized())

		if !r.isNVMLInitialized() {
			log.Info("NVML not initialized, attempting initialization")
			if err := r.InitializeNVML(); err != nil {
				return fmt.Errorf("failed to initialize NVML: %v", err)
			}
			log.Info("NVML initialized successfully")
		}

		// Get GPU count and information
		count, ret := nvml.DeviceGetCount()
		if ret != nvml.SUCCESS {
			return fmt.Errorf("failed to get GPU count: %v", ret)
		}
		log.Info("Found GPUs", "count", count)

		// Create GPU frequency specs
		var gpuFreqs []v1alpha1.GPUFrequencySpec
		for i := 0; i < count; i++ {
			device, ret := nvml.DeviceGetHandleByIndex(i)
			if ret != nvml.SUCCESS {
				log.Error(fmt.Errorf("failed to get device handle"), "GPU index", i)
				continue
			}

			uuid, ret := device.GetUUID()
			if ret != nvml.SUCCESS {
				log.Error(fmt.Errorf("failed to get UUID"), "GPU index", i)
				continue
			}

			// Get current frequencies
			currentGraphicsFreq, ret := device.GetClockInfo(nvml.CLOCK_GRAPHICS)
			if ret != nvml.SUCCESS {
				log.Error(fmt.Errorf("failed to get current graphics frequency: %v", ret), "GPU UUID", uuid)
				continue
			}

			currentMemoryFreq, ret := device.GetClockInfo(nvml.CLOCK_MEM)
			if ret != nvml.SUCCESS {
				log.Error(fmt.Errorf("failed to get current memory frequency: %v", ret), "GPU UUID", uuid)
				continue
			}

			gpuFreqs = append(gpuFreqs, v1alpha1.GPUFrequencySpec{
				UUID:              uuid,
				GraphicsFrequency: int32(currentGraphicsFreq),
				MemoryFrequency:   int32(currentMemoryFreq),
			})

			log.Info("Added GPU to spec",
				"UUID", uuid,
				"GraphicsFreq", currentGraphicsFreq,
				"MemoryFreq", currentMemoryFreq)
		}

		// Create a new NodeFrequencies CR with discovered GPU information
		newNodeFreq := &v1alpha1.NodeFrequencies{
			ObjectMeta: metav1.ObjectMeta{
				Name:      r.NodeName,
				Namespace: r.Namespace,
			},
			Spec: v1alpha1.NodeFrequenciesSpec{
				NodeName:       r.NodeName,
				GPUFrequencies: gpuFreqs,
				CPUFrequencies: []v1alpha1.CPUFrequencySpec{},
			},
			Status: v1alpha1.NodeFrequenciesStatus{
				NodeName:       r.NodeName,
				GPUFrequencies: gpuFreqs,
				CPUFrequencies: []v1alpha1.CPUFrequencySpec{},
			},
		}

		// Get CPU information and update both spec and status
		cpuDirs, err := os.ReadDir(cpufreqBasePath)
		if err != nil {
			log.Error(err, "Failed to read CPU directories")
		} else {
			for _, dir := range cpuDirs {
				if !strings.HasPrefix(dir.Name(), "cpu") {
					continue
				}

				coreID, err := strconv.Atoi(strings.TrimPrefix(dir.Name(), "cpu"))
				if err != nil {
					continue
				}

				// Skip non-numeric CPU directories
				cpufreqPath := filepath.Join(cpufreqBasePath, dir.Name(), "cpufreq")
				if _, err := os.Stat(cpufreqPath); err != nil {
					continue
				}

				// Read current frequency for status
				currentFreqPath := filepath.Join(cpufreqPath, "scaling_cur_freq")
				freqBytes, err := ioutil.ReadFile(currentFreqPath)
				var currentFreq int32
				if err == nil {
					if freqKHz, err := strconv.ParseInt(strings.TrimSpace(string(freqBytes)), 10, 64); err == nil {
						currentFreq = int32(freqKHz / 1000) // Convert kHz to MHz
					}
				}

				// Update both spec and status
				newNodeFreq.Spec.CPUFrequencies = append(newNodeFreq.Spec.CPUFrequencies,
					v1alpha1.CPUFrequencySpec{
						CoreID:    int32(coreID),
						Frequency: currentFreq,
					})

				newNodeFreq.Status.CPUFrequencies = append(newNodeFreq.Status.CPUFrequencies,
					v1alpha1.CPUFrequencySpec{
						CoreID:    int32(coreID),
						Frequency: currentFreq,
					})

				log.Info("Added CPU core",
					"coreID", coreID,
					"currentFreq", currentFreq)
			}
		}

		if err := r.Client.Create(ctx, newNodeFreq); err != nil {
			return fmt.Errorf("failed to create NodeFrequencies CR for node %s: %v", r.NodeName, err)
		}

		log.Info("Created NodeFrequencies CR with GPU and CPU information",
			"nodeName", r.NodeName,
			"namespace", r.Namespace,
			"gpuCount", len(gpuFreqs))
	} else if err != nil {
		return fmt.Errorf("failed to get NodeFrequencies CR for node %s: %v", r.NodeName, err)
	}

	return nil
}

func (r *NodeFrequenciesReconciler) updateStatus(ctx context.Context, nodeFreq *v1alpha1.NodeFrequencies) error {
	log := log.FromContext(ctx)

	// Create new status with current node name
	newStatus := v1alpha1.NodeFrequenciesStatus{
		NodeName: r.NodeName,
	}

	// Update GPU frequencies in status
	for _, gpuSpec := range nodeFreq.Spec.GPUFrequencies {
		device, err := getGPUByUUID(gpuSpec.UUID)
		if err != nil {
			log.Error(err, "Failed to get GPU handle", "UUID", gpuSpec.UUID)
			continue
		}

		// Read current graphics clock
		graphicsFreq, ret := device.GetClockInfo(nvml.CLOCK_GRAPHICS)
		if ret != nvml.SUCCESS {
			log.Error(fmt.Errorf("NVML error: %v", ret), "Failed to get graphics clock", "UUID", gpuSpec.UUID)
			continue
		}

		// Read current memory clock
		memoryFreq, ret := device.GetClockInfo(nvml.CLOCK_MEM)
		if ret != nvml.SUCCESS {
			log.Error(fmt.Errorf("NVML error: %v", ret), "Failed to get memory clock", "UUID", gpuSpec.UUID)
			continue
		}

		newStatus.GPUFrequencies = append(newStatus.GPUFrequencies, v1alpha1.GPUFrequencySpec{
			UUID:              gpuSpec.UUID,
			GraphicsFrequency: int32(graphicsFreq),
			MemoryFrequency:   int32(memoryFreq),
		})

		log.Info("Read GPU frequencies",
			"UUID", gpuSpec.UUID,
			"GraphicsFreq", graphicsFreq,
			"MemoryFreq", memoryFreq)
	}

	// Update CPU frequencies in status
	for _, cpuSpec := range nodeFreq.Spec.CPUFrequencies {
		cpuPath := filepath.Join(cpufreqBasePath, fmt.Sprintf("cpu%d/cpufreq", cpuSpec.CoreID))
		currentFreqPath := filepath.Join(cpuPath, "scaling_cur_freq")

		freqBytes, err := ioutil.ReadFile(currentFreqPath)
		if err != nil {
			log.Error(err, "Failed to read CPU frequency", "CoreID", cpuSpec.CoreID)
			continue
		}

		// Convert kHz to MHz
		freqKHz, err := strconv.ParseInt(strings.TrimSpace(string(freqBytes)), 10, 64)
		if err != nil {
			log.Error(err, "Failed to parse CPU frequency", "CoreID", cpuSpec.CoreID)
			continue
		}
		freqMHz := int32(freqKHz / 1000)

		newStatus.CPUFrequencies = append(newStatus.CPUFrequencies, v1alpha1.CPUFrequencySpec{
			CoreID:    cpuSpec.CoreID,
			Frequency: freqMHz,
		})

		log.Info("Read CPU frequency",
			"CoreID", cpuSpec.CoreID,
			"Frequency", freqMHz)
	}

	// Update the status
	nodeFreq.Status = newStatus
	return r.Status().Update(ctx, nodeFreq)
}

func (r *NodeFrequenciesReconciler) initializeFrequencyCaches() error {
	r.cacheMutex.Lock()
	defer r.cacheMutex.Unlock()

	// Initialize GPU frequency cache from config file
	configPath := "config/gpu_frequencies.json"
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read GPU configs: %v", err)
	}

	var configs map[string]struct {
		SupportedFrequencies []uint32 `json:"supported_frequencies"`
	}
	if err := json.Unmarshal(data, &configs); err != nil {
		return fmt.Errorf("failed to parse GPU configs: %v", err)
	}

	// Get GPU count and cache frequencies for each GPU
	count, ret := nvml.DeviceGetCount()
	if ret == nvml.SUCCESS {
		for i := 0; i < count; i++ {
			device, ret := nvml.DeviceGetHandleByIndex(i)
			if ret != nvml.SUCCESS {
				continue
			}

			uuid, ret := device.GetUUID()
			if ret != nvml.SUCCESS {
				continue
			}

			name, ret := device.GetName()
			if ret != nvml.SUCCESS {
				continue
			}

			if config, exists := configs[name]; exists {
				r.gpuFreqCache[uuid] = config.SupportedFrequencies
			} else {
				_, memClocks, ret := device.GetSupportedMemoryClocks()
				if ret == nvml.SUCCESS {
					memClock := memClocks[0]

					// Get supported graphics clocks for this memory clock
					graphicsCount, graphicsClocks, ret := device.GetSupportedGraphicsClocks(int(memClock))
					if ret == nvml.SUCCESS {
						r.gpuFreqCache[uuid] = make([]uint32, graphicsCount)
						ctrl.Log.Info("GPU frequencies available:", "UUID", uuid, "count", graphicsCount)
						for i, graphicsClock := range graphicsClocks {
							fmt.Printf("  %d MHz\n", graphicsClock)
							r.gpuFreqCache[uuid][i] = graphicsClock
						}
					}
				}
			}
			ctrl.Log.Info("Available GPU frequencies",
				"UUID", uuid,
				"Model", name,
				"FrequencyCount", len(r.gpuFreqCache[uuid]),
				"SupportedFrequencies", r.gpuFreqCache[uuid])

		}
	}

	// Initialize CPU frequency cache
	dirs, err := os.ReadDir(cpufreqBasePath)
	if err != nil {
		return fmt.Errorf("failed to read CPU directories: %v", err)
	}
	for _, dir := range dirs {
		if !strings.HasPrefix(dir.Name(), "cpu") {
			continue
		}
		coreID, err := strconv.Atoi(strings.TrimPrefix(dir.Name(), "cpu"))
		if err != nil {
			continue
		}

		// Check if cpufreq exists for this CPU
		cpufreqPath := filepath.Join(cpufreqBasePath, dir.Name(), "cpufreq")
		if _, err := os.Stat(cpufreqPath); err == nil {
			freqPath := filepath.Join(cpufreqPath, "scaling_available_frequencies")
			if freqBytes, err := os.ReadFile(freqPath); err == nil {
				var freqs []uint32
				for _, f := range strings.Fields(string(freqBytes)) {
					if freq, err := strconv.ParseUint(f, 10, 32); err == nil {
						freqs = append(freqs, uint32(freq))
					}
				}
				r.cpuFreqCache[int32(coreID)] = freqs
			}
		} else {
			r.cpuFreqCache[int32(coreID)] = []uint32{}
			ctrl.Log.Info("CPU frequency scaling not available", "coreID", coreID)
		}
	}

	return nil
}

func (r *NodeFrequenciesReconciler) NodeFrequenciesCRExists(ctx context.Context) (bool, error) {
	log := ctrl.Log.WithName("nf-cr-check")

	var nodeFreq v1alpha1.NodeFrequencies
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      r.NodeName,
		Namespace: r.Namespace,
	}, &nodeFreq)

	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("NodeFrequencies CR not found",
				"nodeName", r.NodeName,
				"namespace", r.Namespace)
			return false, nil
		}
		log.Error(err, "Failed to check NodeFrequencies CR existence",
			"nodeName", r.NodeName,
			"namespace", r.Namespace)
		return false, err
	}

	log.Info("NodeFrequencies CR exists",
		"nodeName", r.NodeName,
		"namespace", r.Namespace)
	return true, nil
}
