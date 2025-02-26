/*

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CronPersistentVolumeAutoscalerSpec defines the desired state of CronPersistentVolumeAutoscaler
type CronPersistentVolumeAutoscalerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of CronPersistentVolumeAutoscaler. Edit CronPersistentVolumeAutoscaler_types.go to remove/update
	TargetRef TargetRef `json:"targetRef,omitempty"`
	Enable    bool      `json:"enable,omitempty"`
	CronTabs  []CronTab `json:"cronTabs,omitempty"`
}

type TargetRef struct {
	PvaName string `json:"pvaName"`
}

type CronTab struct {
	Name     string `json:"name"`
	Schedule string `json:"schedule"`
}

// CronPersistentVolumeAutoscalerStatus defines the observed state of CronPersistentVolumeAutoscaler
type CronPersistentVolumeAutoscalerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Conditions []Condition `json:"conditions,omitempty"`
	Phase      string      `json:"phase,omitempty"`
}

type Condition struct {
	Type               string      `json:"type,omitempty"`
	Status             string      `json:"status,omitempty"`
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	Reason             string      `json:"reason ,omitempty"`
	Message            string      `json:"message,omitempty"`
}

// +genclient
// +kubebuilder:subresource:status
// +kubebuilder:object:root=true
// +kubebuilder:storagevsersion
// +kubebuilder:resource:shortName=cp
// +kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.phase`

type CronPersistentVolumeAutoscaler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CronPersistentVolumeAutoscalerSpec   `json:"spec,omitempty"`
	Status CronPersistentVolumeAutoscalerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// CronPersistentVolumeAutoscalerList contains a list of CronPersistentVolumeAutoscaler
type CronPersistentVolumeAutoscalerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CronPersistentVolumeAutoscaler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CronPersistentVolumeAutoscaler{}, &CronPersistentVolumeAutoscalerList{})
}
