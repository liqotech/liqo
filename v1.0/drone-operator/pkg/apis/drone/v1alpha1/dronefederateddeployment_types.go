package v1alpha1

import (
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PhasePending = "PENDING"
	PhaseScheduling = "SCHEDULING"
	PhaseRunning = "RUNNING"
	PhaseDone    = "DONE"
)

// DroneFederatedDeploymentSpec defines the desired state of DroneFederatedDeployment
// +k8s:openapi-gen=true
type DroneFederatedDeploymentSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Schedule is the desired time the command is supposed to be executed.
	// Note: the format used here is UTC time https://www.utctime.net
	Schedule  string                            `json:"schedule,omitempty"`
	Template v1.Deployment						`json:"template,omitempty"`
	//Template  DroneFederatedDeploymentTemplate  `json:"template,omitempty"`
	Placement DroneFederatedDeploymentPlacement `json:"placement,omitempty"`
	Overrides DroneFederatedDeploymentOverrides `json:"overrides,omitempty"`
}

/*
type DroneFederatedDeploymentTemplate struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DroneFederatedDeploymentTemplateSpec `json:"spec,omitempty"`
}

type DroneFederatedDeploymentTemplateSpec struct {
	Replicas int64                                        `json:"replicas,omitempty"`
	Selector DroneFederatedDeploymentTemplateSpecSelector `json:"selector,omitempty"`
}
type DroneFederatedDeploymentTemplateSpecSelector struct {
	MatchLabels DroneFederatedDeploymentTemplateSpecSelectorMatchLabels `json:"matchLabels,omitempty"`
}
type DroneFederatedDeploymentTemplateSpecSelectorMatchLabels struct {
	App string `json:"app,omitempty"`
}
type DroneFederatedDeploymentTemplateSpecTemplate struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DroneFederatedDeploymentTemplateSpecTemplateSpec `json:"spec,omitempty"`
}
type DroneFederatedDeploymentTemplateSpecTemplateSpec struct {
	Containers DroneFederatedDeploymentTemplateSpecTemplateSpecContainers `json:"containers,omitempty"`
}
type DroneFederatedDeploymentTemplateSpecTemplateSpecContainers struct {
	image     string                                                              `json:"image,omitempty"`
	Name      string                                                              `json:"name,omitempty"`
	Resources DroneFederatedDeploymentTemplateSpecTemplateSpecContainersResources `json:"resources,omitempty"`
}

type DroneFederatedDeploymentTemplateSpecTemplateSpecContainersResources struct {
	Limits Limit `json:"limits,omitempty"`
}

type Limit struct{
	Memory string `json:"memory,omitempty"`
	Cpu    string `json:"cpu,omitempty"`
}
*/

type DroneFederatedDeploymentPlacement struct {
	Clusters []DroneFederatedDeploymentPlacementClusters `json:"clusters,omitempty"`
}

type DroneFederatedDeploymentPlacementClusters struct {
	Name string `json:"name,omitempty"`
}

type DroneFederatedDeploymentOverrides struct {
}

/*
// DroneFederatedDeploymentSpec defines the desired state of DroneFederatedDeployment
// +k8s:openapi-gen=true
type DroneFederatedDeploymentSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Schedule is the desired time the command is supposed to be executed.
	// Note: the format used here is UTC time https://www.utctime.net
	Schedule   string                              `json:"schedule,omitempty"`
	AppName    string                              `json:"app-name,omitempty"`
	BaseNode   string                              `json:"base-node,omitempty"`
	Type       string                              `json:"type,omitempty"`
	Components []DroneFederatedDeploymentComponent `json:"components,omitempty"`
}

type DroneFederatedDeploymentComponent struct {
	Name             string                                    `json:"name,omitempty"`
	Function         DroneFederatedDeploymentComponentFunction `json:"function,omitempty"`
	//Parameters       interface{}                               `json:"parameters"`
	BootDependencies []string                                  `json:"boot_dependencies,omitempty"`
	NodesBlacklist   []string                                  `json:"nodes-blacklist,omitempty"`
	NodesWhitelist   []string                                  `json:"nodes-whitelist,omitempty"`
}

type DroneFederatedDeploymentComponentFunction struct {
	Image    string                                             `json:"image,omitempty"`
	Resources DroneFederatedDeploymentComponentFunctionResources `json:"resources,omitempty"`
}

type DroneFederatedDeploymentComponentFunctionResources struct {
	Memory float64 `json:"memory,omitempty"`
	Cpu    float64 `json:"cpu,omitempty"`
}
*/
// DroneFederatedDeploymentStatus defines the observed state of DroneFederatedDeployment
// +k8s:openapi-gen=true
type DroneFederatedDeploymentStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Phase represents the state of the schedule:
	//		initial is SCHEDULING, until message is sent
	// 		until the deploy is executed it is PENDING,
	// 		afterwards it is RUNNING,
	//		end it is DONE
	Phase string `json:"phase,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DroneFederatedDeployment is the Schema for the dronefederateddeployments API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
type DroneFederatedDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DroneFederatedDeploymentSpec   `json:"spec,omitempty"`
	Status DroneFederatedDeploymentStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DroneFederatedDeploymentList contains a list of DroneFederatedDeployment
type DroneFederatedDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DroneFederatedDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&DroneFederatedDeployment{}, &DroneFederatedDeploymentList{})
}
