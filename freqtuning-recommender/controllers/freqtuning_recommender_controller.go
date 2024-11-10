package controllers

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	freqtunerv1alpha1 "github.com/Climatik-Project/Climatik-Project/freqtuner/api/v1alpha1"
	"github.com/Climatik-Project/Climatik-Project/freqtuning-recommender/pkg/algorithms"
	climatikv1alpha1 "github.com/Climatik-Project/Climatik-Project/powercapping-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	defaultRecommenderName = "freqtuning-recommender"
	defaultNodeFrequencyNS = "climatik-project"
	reconcileInterval      = time.Minute
)

var (
	recommenderName = func() string {
		if name := os.Getenv("RECOMMENDER_NAME"); name != "" {
			return name
		}
		return defaultRecommenderName
	}()

	nodeFrequencyNamespace = func() string {
		if ns := os.Getenv("NODE_FREQUENCY_NAMESPACE"); ns != "" {
			return ns
		}
		return defaultNodeFrequencyNS
	}()
)

// FreqTuningRecommenderReconciler reconciles PowerCappingPolicy objects
type FreqTuningRecommenderReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Config     *rest.Config
	KubeClient kubernetes.Interface
}

// +kubebuilder:rbac:groups=climatik.io,resources=powercappingpolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=climatik.io,resources=nodefrequencies,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

func (r *FreqTuningRecommenderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Add request details
	log.Info("=== Starting reconciliation ===",
		"request.namespace", req.Namespace,
		"request.name", req.Name)

	// Get all PowerCappingPolicies
	var policyList climatikv1alpha1.PowerCappingPolicyList
	if err := r.List(ctx, &policyList, client.InNamespace("")); err != nil {
		log.Error(err, "Failed to list policies")
		return ctrl.Result{}, err
	}
	log.Info("Found policies",
		"count", len(policyList.Items),
		"policies", getPolicyNames(policyList.Items))

	// Process each policy
	for _, policy := range policyList.Items {
		log.Info("=== Processing policy ===",
			"name", policy.Name,
			"namespace", policy.Namespace,
			"powerCapLimit", policy.Spec.PowerCapLimit,
			"cappingThreshold", policy.Spec.CappingThreshold,
			"algorithms", getAlgorithmNames(policy.Spec.CustomAlgorithms))

		// Check if this recommender should handle this policy
		shouldHandle := false
		for _, algo := range policy.Spec.CustomAlgorithms {
			log.Info("Checking algorithm",
				"algo", algo.Name,
				"recommender", recommenderName)
			if algo.Name == recommenderName {
				shouldHandle = true
				break
			}
		}
		if !shouldHandle {
			continue
		}

		// Check if actions are required
		if !policy.Status.CappingActionRequired {
			log.Info("No capping actions are required at this time")
			continue
		}

		// Get GPU resources for pods matching the policy
		nodeResources, err := r.getGPUResourcesFromPods(ctx, &policy)
		if err != nil {
			log.Error(err, "Failed to get GPU resources")
			continue
		}

		if len(nodeResources) == 0 {
			log.Info("No resources found for policy pods")
			continue
		}

		// Calculate required frequency adjustments
		nodeFreqs, err := r.calculateFrequencyAdjustments(ctx, nodeResources, &policy)
		if err != nil {
			log.Error(err, "Failed to calculate frequency adjustments")
			continue
		}

		// Update NodeFrequency CRs
		if err := r.updateNodeFrequencies(ctx, nodeFreqs); err != nil {
			log.Error(err, "Failed to update NodeFrequency CRs")
			continue
		}

		log.Info("Successfully updated all NodeFrequency CRs")
	}

	// Return with RequeueAfter to ensure periodic reconciliation
	return ctrl.Result{RequeueAfter: reconcileInterval}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FreqTuningRecommenderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Config == nil {
		return fmt.Errorf("Config cannot be nil")
	}

	kubeClient, err := kubernetes.NewForConfig(r.Config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}
	r.KubeClient = kubeClient

	// Load and cache power profile once during startup
	if err := algorithms.LoadAndCachePowerProfile(context.Background()); err != nil {
		return fmt.Errorf("failed to load power profile: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&climatikv1alpha1.PowerCappingPolicy{}).
		Complete(r)
}

// getGPUResourcesFromPods returns a map of node names to GPU UUIDs for pods matching the policy selector
func (r *FreqTuningRecommenderReconciler) getGPUResourcesFromPods(ctx context.Context, policy *climatikv1alpha1.PowerCappingPolicy) (map[string][]string, error) {
	log := log.FromContext(ctx)
	nodeResources := make(map[string][]string)

	// Get pods matching the selector
	var podList corev1.PodList
	if err := r.List(ctx, &podList, client.MatchingLabels(policy.Spec.Selector.MatchLabels)); err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	// Get nodes and their resources where pods are running
	for _, pod := range podList.Items {
		nodeName := pod.Spec.NodeName
		if nodeName == "" {
			continue
		}

		// Check containers for GPU usage
		for _, container := range pod.Spec.Containers {
			// Check if container requests NVIDIA GPUs
			if _, exists := container.Resources.Limits["nvidia.com/gpu"]; exists {
				log.Info("Found container with GPU request",
					"pod", pod.Name,
					"container", container.Name,
					"node", nodeName)

				// Get container environment variables using exec
				req := r.KubeClient.CoreV1().RESTClient().Post().
					Resource("pods").
					Name(pod.Name).
					Namespace(pod.Namespace).
					SubResource("exec").
					VersionedParams(&corev1.PodExecOptions{
						Container: container.Name,
						Command:   []string{"env"},
						Stdout:    true,
						Stderr:    true,
					}, scheme.ParameterCodec)

				exec, err := remotecommand.NewSPDYExecutor(r.Config, "POST", req.URL())
				if err != nil {
					log.Error(err, "Failed to create executor",
						"pod", pod.Name,
						"container", container.Name)
					continue
				}

				var stdout, stderr strings.Builder
				if err := exec.Stream(remotecommand.StreamOptions{
					Stdout: &stdout,
					Stderr: &stderr,
				}); err != nil {
					log.Error(err, "Failed to execute command",
						"pod", pod.Name,
						"container", container.Name,
						"stderr", stderr.String())
					continue
				}

				// Parse environment variables
				envVars := strings.Split(stdout.String(), "\n")
				for _, env := range envVars {
					if strings.HasPrefix(env, "NVIDIA_VISIBLE_DEVICES=") {
						gpuUUID := strings.TrimPrefix(env, "NVIDIA_VISIBLE_DEVICES=")
						gpuUUID = strings.TrimSpace(gpuUUID)

						// Initialize node's GPU list if not exists
						if _, ok := nodeResources[nodeName]; !ok {
							nodeResources[nodeName] = []string{}
						}

						// Split in case multiple GPUs are assigned
						gpuUUIDs := strings.Split(gpuUUID, ",")
						for _, uuid := range gpuUUIDs {
							uuid = strings.TrimSpace(uuid)
							if uuid != "" && !contains(nodeResources[nodeName], uuid) {
								nodeResources[nodeName] = append(nodeResources[nodeName], uuid)
								log.Info("Found GPU device",
									"pod", pod.Name,
									"container", container.Name,
									"node", nodeName,
									"uuid", uuid)
							}
						}
						break
					}
				}
			}
		}
	}

	if len(nodeResources) == 0 {
		log.Info("No GPU resources found for policy pods")
	} else {
		log.Info("Found GPU resources", "nodeResources", nodeResources)
	}

	return nodeResources, nil
}

// NodeGPUFrequencies maps node names to a map of GPU UUIDs and their target frequencies
type NodeGPUFrequencies map[string]map[string]int32

func (r *FreqTuningRecommenderReconciler) calculateFrequencyAdjustments(ctx context.Context, nodeResources map[string][]string, policy *climatikv1alpha1.PowerCappingPolicy) (NodeGPUFrequencies, error) {
	log := log.FromContext(ctx)

	// Create a new scaler for this reconciliation
	scaler := algorithms.NewDynamicFrequencyScaler(
		0.99,
		300,
		float64(policy.Spec.PowerCapLimit), // Set power cap directly
	)

	// Calculate per-GPU power budget
	totalGPUs := 0
	for _, gpus := range nodeResources {
		totalGPUs += len(gpus)
	}
	perGPUPowerBudget := float64(policy.Spec.PowerCapLimit) / float64(totalGPUs)

	// Initialize result structure
	nodeFreqs := make(NodeGPUFrequencies)

	// Calculate frequencies per node and GPU
	for nodeName, gpuUUIDs := range nodeResources {
		nodeFreqs[nodeName] = make(map[string]int32)
		for _, uuid := range gpuUUIDs {
			freq := scaler.CalculateFrequency(ctx, perGPUPowerBudget)
			nodeFreqs[nodeName][uuid] = freq
		}
	}

	log.Info("Calculated frequency adjustments",
		"totalGPUs", totalGPUs,
		"powerCap", policy.Spec.PowerCapLimit,
		"perGPUPowerBudget", perGPUPowerBudget,
		"nodeFrequencies", nodeFreqs)

	return nodeFreqs, nil
}

// updateNodeFrequencies creates or updates NodeFrequency CRs based on calculated frequencies
func (r *FreqTuningRecommenderReconciler) updateNodeFrequencies(ctx context.Context, nodeFreqs NodeGPUFrequencies) error {
	log := log.FromContext(ctx)

	// First, list all existing NodeFrequencies
	var existingNFs freqtunerv1alpha1.NodeFrequenciesList
	if err := r.List(ctx, &existingNFs, client.InNamespace(nodeFrequencyNamespace)); err != nil {
		return fmt.Errorf("failed to list existing NodeFrequencies: %w", err)
	}

	// Create a map of node name to existing NodeFrequency CR name
	nodeToNFName := make(map[string]string)
	for _, nf := range existingNFs.Items {
		if nf.Spec.NodeName != "" {
			nodeToNFName[nf.Spec.NodeName] = nf.Name
		}
	}

	for nodeName, gpuFreqs := range nodeFreqs {
		// Get the correct NodeFrequency name for this node
		nfName, exists := nodeToNFName[nodeName]
		if !exists {
			log.Info("No existing NodeFrequency found for node", "node", nodeName)
			continue
		}

		nodeFreq := &freqtunerv1alpha1.NodeFrequencies{
			ObjectMeta: metav1.ObjectMeta{
				Name:      nfName, // Use the existing NF name instead of node name
				Namespace: nodeFrequencyNamespace,
			},
		}

		// Create or update the NodeFrequencies CR
		op, err := ctrl.CreateOrUpdate(ctx, r.Client, nodeFreq, func() error {
			// Update spec
			nodeFreq.Spec.NodeName = nodeName
			// Update frequencies for existing GPUs
			for uuid, newFreq := range gpuFreqs {
				found := false
				for i := range nodeFreq.Spec.GPUFrequencies {
					if nodeFreq.Spec.GPUFrequencies[i].UUID == uuid {
						nodeFreq.Spec.GPUFrequencies[i].GraphicsFrequency = newFreq
						found = true
						log.Info("Updated GPU frequency",
							"node", nodeName,
							"uuid", uuid,
							"graphicsFrequency", newFreq)
						break
					}
				}
				if !found {
					nodeFreq.Spec.GPUFrequencies = append(nodeFreq.Spec.GPUFrequencies, freqtunerv1alpha1.GPUFrequencySpec{
						UUID:              uuid,
						GraphicsFrequency: newFreq,
					})
					log.Info("Added new GPU frequency",
						"node", nodeName,
						"uuid", uuid,
						"graphicsFrequency", newFreq)
				}
			}
			return nil
		})

		if err != nil {
			log.Error(err, "Failed to create/update NodeFrequencies", "node", nodeName)
			return err
		}

		log.Info("NodeFrequencies CR updated",
			"node", nodeName,
			"operation", op,
			"frequencies", gpuFreqs)
	}

	return nil
}

// Helper functions
func getPolicyNames(policies []climatikv1alpha1.PowerCappingPolicy) []string {
	names := make([]string, len(policies))
	for i, p := range policies {
		names[i] = fmt.Sprintf("%s/%s", p.Namespace, p.Name)
	}
	return names
}

func getAlgorithmNames(algos []climatikv1alpha1.CustomAlgorithm) []string {
	names := make([]string, len(algos))
	for i, a := range algos {
		names[i] = a.Name
	}
	return names
}

func contains(slice []string, str string) bool {
	for _, v := range slice {
		if v == str {
			return true
		}
	}
	return false
}
