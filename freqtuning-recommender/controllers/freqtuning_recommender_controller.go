package controllers

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	climatikv1alpha1 "github.com/Climatik-Project/Climatik-Project/powercapping-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"k8s.io/apimachinery/pkg/types"
	nfdv1alpha1 "sigs.k8s.io/node-feature-discovery/pkg/apis/nfd/v1alpha1"
	"github.com/NVIDIA/go-nvml/pkg/nvml"
	"github.com/Climatik-Project/Climatik-Project/powercapping-controller/pkg/algorithms"
)

const defaultRecommenderName = "freqtuning-recommender"

var recommenderName = func() string {
	if name := os.Getenv("RECOMMENDER_NAME"); name != "" {
		return name
	}
	return defaultRecommenderName
}()

// FreqTuningRecommenderReconciler reconciles PowerCappingPolicy objects
type FreqTuningRecommenderReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=climatik.io,resources=powercappingpolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups=climatik.io,resources=nodefrequencies,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

func (r *FreqTuningRecommenderReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Get all PowerCappingPolicies
	var policyList climatikv1alpha1.PowerCappingPolicyList
	if err := r.List(ctx, &policyList); err != nil {
		log.Error(err, "Failed to list policies")
		return ctrl.Result{}, err
	}

	// Process each policy
	for _, policy := range policyList.Items {
		// Check if this recommender should handle this policy
		shouldHandle := false
		for _, algo := range policy.Spec.CustomAlgorithms {
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

		// TODO: Next steps with nodeResources
		// 2. Calculate required frequency adjustments using DynamicFrequencyScaler
		// 3. Update NodeFrequency CRs
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FreqTuningRecommenderReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&climatikv1alpha1.PowerCappingPolicy{}).
		Watches(
			&corev1.Pod{},
			handler.EnqueueRequestsFromMapFunc(r.findPoliciesForPod),
		).
		Complete(r)
}

// findPoliciesForPod maps pods to their relevant PowerCappingPolicies
func (r *FreqTuningRecommenderReconciler) findPoliciesForPod(ctx context.Context, pod client.Object) []ctrl.Request {
	// TODO: Implement logic to find policies that match this pod's labels
	return []ctrl.Request{}
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
			if _, exists := container.Resources.Limits["nvidia.com/gpu"]; !exists {
				continue
			}

			// Get container status to check environment variables
			for _, status := range pod.Status.ContainerStatuses {
				if status.Name == container.Name {
					// Get GPU UUID from environment variables
					for _, env := range container.Env {
						if env.Name == "NVIDIA_VISIBLE_DEVICES" {
							if _, ok := nodeResources[nodeName]; !ok {
								nodeResources[nodeName] = []string{}
							}
							// Split in case multiple GPUs are assigned
							gpuUUIDs := strings.Split(env.Value, ",")
							nodeResources[nodeName] = append(nodeResources[nodeName], gpuUUIDs...)
							break
						}
					}
				}
			}
		}
	}

	if len(nodeResources) == 0 {
		log.Info("No resources found for policy pods")
	}

	return nodeResources, nil
}

// NodeGPUFrequencies maps node names to a map of GPU UUIDs and their target frequencies
type NodeGPUFrequencies map[string]map[string]int32

func (r *FreqTuningRecommenderReconciler) calculateFrequencyAdjustments(ctx context.Context, nodeResources map[string][]string, policy *climatikv1alpha1.PowerCappingPolicy) (NodeGPUFrequencies, error) {
	log := log.FromContext(ctx)

	scaler := algorithms.NewDynamicFrequencyScaler(
		0.99,
		300,
		policy.Spec.PowerCap,
	)

	// Calculate per-GPU power budget
	totalGPUs := 0
	for _, gpus := range nodeResources {
		totalGPUs += len(gpus)
	}
	perGPUPowerBudget := float64(policy.Spec.PowerCap) / float64(totalGPUs)

	// Initialize result structure
	nodeFreqs := make(NodeGPUFrequencies)
	
	// Calculate frequencies per node and GPU
	for nodeName, gpuUUIDs := range nodeResources {
		nodeFreqs[nodeName] = make(map[string]int32)
		for _, uuid := range gpuUUIDs {
			freq := scaler.CalculateFrequencyForPowerCap(perGPUPowerBudget)
			nodeFreqs[nodeName][uuid] = freq[uuid]
		}
	}

	log.Info("Calculated frequency adjustments",
		"totalGPUs", totalGPUs,
		"powerCap", policy.Spec.PowerCap,
		"perGPUPowerBudget", perGPUPowerBudget,
		"nodeFrequencies", nodeFreqs)

	return nodeFreqs, nil
}
