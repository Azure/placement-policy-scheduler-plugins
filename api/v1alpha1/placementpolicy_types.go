package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type (
	// EnforcementMode is an enumeration of the enforcement modes
	EnforcementMode string
	// Action is an enumeration of the actions
	Action string
)

const (
	// EnforcementModeBestEffort means the policy will be enforced as best effort
	EnforcementModeBestEffort EnforcementMode = "BestEffort"
	// EnforcementModeStrict the policy will be forced during scheduling
	EnforcementModeStrict EnforcementMode = "Strict"

	// ActionMust means the pods must be placed on the node
	ActionMust Action = "Must"
	// ActionMustNot means the pods must not be placed on the node
	ActionMustNot Action = "MustNot"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PlacementPolicySpec defines the desired state of PlacementPolicy
type PlacementPolicySpec struct {
	// The policy weight allows the engine to decide which policy to use when
	// pods match multiple policies. If multiple policies matched and all
	// share the same weight then a policy with spec.enforcementMode == Force
	// will be selected. If multiple policies match and +1 policy is marked
	// as “Force” enforcementMode then they will sorted alphabetically /
	// ascending and first one will be used. The scheduler publishes events
	// capturing this conflict when it happens. Weight == 0-100 is reserved
	// for future use.
	Weight int32 `json:"weight,omitempty"`
	// enforcementMode is an enum that specifies how the policy will be
	// enforced during scheduler (e.g. the application of filter vs scorer
	// plugin). Values allowed for this field are:
	// BestEffort (default): the policy will be enforced as best effort
	// (scorer mode).
	// Strict: the policy will be forced during scheduling. The filter
	// approach will be used. Note: that may yield pods unschedulable.
	EnforcementMode EnforcementMode `json:"enforcementMode,omitempty"`
	// podSelector identifies which pods this placement policy will apply on
	PodSelector *metav1.LabelSelector `json:"podSelector,omitempty"`
	// nodeSelector selects the nodes where the placement policy will
	// apply on according to action
	NodeSelector *metav1.LabelSelector `json:"nodeSelector,omitempty"`
	// Policy is the policy placement for target based on action
	Policy *Policy `json:"policy,omitempty"`
}

type Policy struct {
	// The action field is policy placement action. It is a string enum
	// that carries the following possible values:
	// Must(default): based on the rule below pods must be placed on
	// nodes selected by node selector
	// MustNot: based on the rule pods must *not* be placed nodes
	// selected by node selector
	Action Action `json:"action,omitempty"`
	// TargetSize is the number of pods that can or cannot be placed on the node.
	// Value can be an absolute number (ex: 5) or a percentage of desired pods (ex: 10%).
	// Absolute number is calculated from percentage by rounding down.
	TargetSize *intstr.IntOrString `json:"targetSize,omitempty"`
}

// PlacementPolicyStatus defines the observed state of PlacementPolicy
type PlacementPolicyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// PlacementPolicy is the Schema for the placementpolicies API
type PlacementPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PlacementPolicySpec   `json:"spec,omitempty"`
	Status PlacementPolicyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PlacementPolicyList contains a list of PlacementPolicy
type PlacementPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PlacementPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PlacementPolicy{}, &PlacementPolicyList{})
}
