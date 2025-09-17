package v1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Wtf
type RawValueSource struct {
	// An inline value object.
	// This field is mutually exclusive with `configMap`, and is effectively required if `configMap` is not defined.
	Value *runtime.RawExtension `json:"value,omitempty"`
	// A reference to a ConfigMap key containing a value encoded as a JSON string.
	// This field is mutually exclusive with `value`, and is effectively required if `value` is not defined.
	ConfigMap *corev1.ConfigMapKeySelector `json:"configMap,omitempty"`
	// A list of RFC-6902 JSON patches to apply to this value.
	Patches []JsonPatch `json:"patches,omitempty"`
}

// An [RFC-6902](https://datatracker.ietf.org/doc/html/rfc6902) JSON patch
type JsonPatch struct {
	// A JSON patch operation
	Operation string `json:"op"`
	// A string containing an RFC-6901] value that references a location within
	// the target document where the operation will be performed
	Path string `json:"path"`

	// Value to be added/edited by the patch operation
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
