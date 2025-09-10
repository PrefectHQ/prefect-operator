package v1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type RawValueSource struct {
	Value     *runtime.RawExtension        `json:"value,omitempty"`
	ConfigMap *corev1.ConfigMapKeySelector `json:"configMap,omitempty"`
	Patches   []JsonPatch                  `json:"patches,omitempty"`
}

type JsonPatch struct {
	Operation string `json:"op"`
	Path      string `json:"path"`

	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	Value *runtime.RawExtension `json:"value,omitempty"`
}

// TODO - no admission webhook yet
func (spec *RawValueSource) Validate() error {
	if spec.Value != nil && spec.ConfigMap != nil {
		return fmt.Errorf("value and configMap are mutually exclusive")
	}
	return nil
}
