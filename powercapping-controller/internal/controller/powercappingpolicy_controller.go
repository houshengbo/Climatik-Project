/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	climatikv1alpha1 "github.com/Climatik-Project/Climatik-Project/powercapping-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PowerCappingPolicyReconciler reconciles a PowerCappingPolicy object
type PowerCappingPolicyReconciler struct {
	client.Client
	Log           logr.Logger
	Scheme        *runtime.Scheme
	CheckInterval time.Duration
	PrometheusURL string
}

//+kubebuilder:rbac:groups=climatik.io,resources=powercappingpolicies,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=climatik.io,resources=powercappingpolicies/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=climatik.io,resources=powercappingpolicies/finalizers,verbs=update

func (r *PowerCappingPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("powercappingpolicy", req.NamespacedName)

	var policy climatikv1alpha1.PowerCappingPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		log.Error(err, "Unable to fetch PowerCappingPolicy")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Implement power usage monitoring logic
	cappingRequired, err := r.checkPowerUsage(&policy)
	if err != nil {
		log.Error(err, "Failed to check power usage")
		return ctrl.Result{}, err
	}

	// Update status
	currentTime := time.Now()
	log.Info("Capping status update",
		"timestamp", currentTime.Format(time.RFC3339),
		"cappingRequired", cappingRequired)
	policy.Status.CappingActionRequired = cappingRequired
	policy.Status.LastUpdated = metav1.Now()
	if err := r.Status().Update(ctx, &policy); err != nil {
		log.Error(err, "Failed to update PowerCappingPolicy status")
		return ctrl.Result{}, err
	}

	// Requeue the request to periodically check power usage
	return ctrl.Result{RequeueAfter: r.CheckInterval}, nil
}

func (r *PowerCappingPolicyReconciler) checkPowerUsage(policy *climatikv1alpha1.PowerCappingPolicy) (bool, error) {
	// Make sure the URL has the correct format
	if !strings.HasPrefix(r.PrometheusURL, "http://") && !strings.HasPrefix(r.PrometheusURL, "https://") {
		r.PrometheusURL = "http://" + r.PrometheusURL
	}

	// Increase timeout to 30 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := api.NewClient(api.Config{
		Address:      r.PrometheusURL,
		RoundTripper: api.DefaultRoundTripper,
	})
	if err != nil {
		return false, fmt.Errorf("error creating Prometheus client: %v", err)
	}

	v1api := v1.NewAPI(client)

	// Build label selector string from policy.Spec.Selector.MatchLabels
	labelSelectors := []string{}
	for _, value := range policy.Spec.Selector.MatchLabels {
		labelSelectors = append(labelSelectors, fmt.Sprintf(`exported_pod=~'.*%s.*'`, value))
	}
	labelSelectorStr := fmt.Sprintf("{%s}", strings.Join(labelSelectors, ","))

	// Query that sums power usage across all matching pods
	query := fmt.Sprintf("sum(DCGM_FI_DEV_POWER_USAGE%s)", labelSelectorStr)

	result, warnings, err := v1api.Query(ctx, query, time.Now())
	if err != nil {
		r.Log.Error(err, "Query failed")
		return false, fmt.Errorf("error querying Prometheus: %v", err)
	}

	if len(warnings) > 0 {
		fmt.Printf("Warnings: %v\n", warnings)
	}

	// Process the query result
	var currentPowerUsage float64
	if vector, ok := result.(model.Vector); ok {
		if len(vector) > 0 {
			currentPowerUsage = float64(vector[0].Value)
		}
	}

	cappingThreshold := float64(policy.Spec.CappingThreshold) / 100.0
	cappingLimit := float64(policy.Spec.PowerCapLimit) * cappingThreshold

	r.Log.Info("Power usage check",
		"currentPowerUsage", currentPowerUsage,
		"cappingLimit", cappingLimit,
		"cappingThreshold", cappingThreshold,
		"rawPowerCapLimit", policy.Spec.PowerCapLimit)

	return currentPowerUsage > cappingLimit, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *PowerCappingPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Log = mgr.GetLogger().WithName("controllers").WithName("PowerCappingPolicy")

	// Read from env var with default fallback
	interval := os.Getenv("POWER_CHECK_INTERVAL")
	if interval == "" {
		r.CheckInterval = 1 * time.Minute // default
	} else {
		duration, err := time.ParseDuration(interval)
		if err != nil {
			r.Log.Error(err, "Invalid check interval format, using default", "input", interval)
			r.CheckInterval = 1 * time.Minute
		} else {
			r.CheckInterval = duration
		}
	}

	r.Log.Info("Configured power check interval", "duration", r.CheckInterval)
	return ctrl.NewControllerManagedBy(mgr).
		For(&climatikv1alpha1.PowerCappingPolicy{}).
		Complete(r)
}
