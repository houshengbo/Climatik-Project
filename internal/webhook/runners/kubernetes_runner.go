package runners

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

type KubernetesRunner struct {
	JobManifestPath string
}

func (r *KubernetesRunner) Run() error {
	// Load the Kubernetes configuration
	var kubeconfig string
	if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return fmt.Errorf("failed to load kubeconfig: %v", err)
	}

	// Create a new Kubernetes client
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %v", err)
	}

	// Read the job manifest
	manifest, err := ioutil.ReadFile(r.JobManifestPath)
	if err != nil {
		return fmt.Errorf("failed to read job manifest: %v", err)
	}

	// Create a Job object from the manifest
	job := &batchv1.Job{}
	if err := json.Unmarshal(manifest, job); err != nil {
		return fmt.Errorf("failed to unmarshal job manifest: %v", err)
	}

	// Create the Job in the Kubernetes cluster
	jobClient := clientset.BatchV1().Jobs("default")
	createdJob, err := jobClient.Create(context.TODO(), job, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes job: %v", err)
	}

	fmt.Printf("Job %s created successfully\n", createdJob.Name)
	return nil
}
