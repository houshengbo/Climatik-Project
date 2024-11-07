// +groupName=freqtuner.climatik.io

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NodeFrequenciesSpec defines the desired state of NodeFrequencies
// +k8s:deepcopy-gen=true
type NodeFrequenciesSpec struct {
	// NodeName is the name of the node this CR corresponds to
	// +kubebuilder:validation:Required
	NodeName string `json:"nodeName"`

	// GPUFrequencies is a list of desired GPU frequencies
	GPUFrequencies []GPUFrequencySpec `json:"gpuFrequencies,omitempty"`

	// CPUFrequencies is a list of desired CPU frequencies
	CPUFrequencies []CPUFrequencySpec `json:"cpuFrequencies,omitempty"`
}

// NodeFrequenciesStatus defines the observed state of NodeFrequencies
// +k8s:deepcopy-gen=true
type NodeFrequenciesStatus struct {
	// NodeName is the name of the node this CR corresponds to
	NodeName string `json:"nodeName"`

	// GPUFrequencies is a list of observed GPU frequencies
	GPUFrequencies []GPUFrequencySpec `json:"gpuFrequencies,omitempty"`

	// CPUFrequencies is a list of observed CPU frequencies
	CPUFrequencies []CPUFrequencySpec `json:"cpuFrequencies,omitempty"`
}

// GPUFrequencySpec defines the desired frequency for a GPU
// +k8s:deepcopy-gen=true
type GPUFrequencySpec struct {
	// UUID is the unique identifier of the GPU
	// +kubebuilder:validation:Required
	UUID string `json:"uuid"`

	// GraphicsFrequency is the desired graphics clock frequency in MHz
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=2500
	GraphicsFrequency int32 `json:"graphicsFrequency,omitempty"`

	// MemoryFrequency is the desired memory clock frequency in MHz
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=2500
	MemoryFrequency int32 `json:"memoryFrequency,omitempty"`
}

// CPUFrequencySpec defines the desired frequency for a CPU core
type CPUFrequencySpec struct {
	// CoreID is the ID of the CPU core
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=0
	CoreID int32 `json:"coreId"`

	// Frequency is the desired frequency in MHz
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=5000
	Frequency int32 `json:"frequency,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced,shortName=nf,path=nodefrequencies,singular=nodefrequency
//+kubebuilder:printcolumn:name="Node",type="string",JSONPath=".spec.nodeName"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// NodeFrequencies is the Schema for the nodefrequencies API
type NodeFrequencies struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NodeFrequenciesSpec   `json:"spec,omitempty"`
	Status NodeFrequenciesStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// NodeFrequenciesList contains a list of NodeFrequencies
type NodeFrequenciesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NodeFrequencies `json:"items"`
}

func init() {
	SchemeBuilder.Register(&NodeFrequencies{}, &NodeFrequenciesList{})
}
