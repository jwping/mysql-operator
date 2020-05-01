package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MysqlOperatorSpec defines the desired state of MysqlOperator
type MysqlOperatorSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	Mysql ControllerSpec `json:"mysql"`
}

type ControllerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html
	Size        int32                       `json:"size"`
	Image       string                      `json:"image"`
	Resources   corev1.ResourceRequirements `json:"resources,omitempty"`
	Envs        []corev1.EnvVar             `json:"envs,omitempty"`
	Ports       []corev1.ServicePort        `json:"ports,omitempty"`
	MultiMaster bool                        `json:"multiMaster,omitempty"`
}

// MysqlOperatorStatus defines the observed state of MysqlOperator
type MysqlOperatorStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	Nodes                    []string `json:"nodes"`
	appsv1.StatefulSetStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MysqlOperator is the Schema for the mysqloperators API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=mysqloperators,scope=Namespaced
type MysqlOperator struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MysqlOperatorSpec   `json:"spec,omitempty"`
	Status MysqlOperatorStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MysqlOperatorList contains a list of MysqlOperator
type MysqlOperatorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MysqlOperator `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MysqlOperator{}, &MysqlOperatorList{})
}
