/*
Copyright 2025.

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

package v1alpha1

import (
	"errors"
	cachev1beta1 "memcached-operator/api/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MemcachedSpec defines the desired state of Memcached
type MemcachedSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Size is the size of the memcached deployment
	Size uint `json:"size"`
}

// MemcachedStatus defines the observed state of Memcached
type MemcachedStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Nodes are the names of the memcached pods
	Nodes []string `json:"nodes"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Memcached is the Schema for the memcacheds API
type Memcached struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MemcachedSpec   `json:"spec,omitempty"`
	Status MemcachedStatus `json:"status,omitempty"`
}

// ConvertTo converts this version (v1alpha1) to the Hub version (v1beta1)
func (m *Memcached) ConvertTo(rawDestination conversion.Hub) error {
	destination, ok := rawDestination.(*cachev1beta1.Memcached)
	if !ok {
		return errors.New("destination type is not cachev1beta1.Memcached")
	}

	destination.ObjectMeta = m.ObjectMeta
	destination.Spec.DisableEvictions = true
	destination.Spec.Size = m.Spec.Size

	destination.Status.Nodes = m.Status.Nodes

	return nil
}

// ConvertFrom converts from the Hub version (v1beta1) to this version (v1alpha1)
func (m *Memcached) ConvertFrom(rawSource conversion.Hub) error {
	source, ok := rawSource.(*cachev1beta1.Memcached)
	if !ok {
		return errors.New("source type is not cachev1beta1.Memcached")
	}

	m.ObjectMeta = source.ObjectMeta

	m.Spec.Size = source.Spec.Size

	m.Status.Nodes = source.Status.Nodes

	return nil
}

//+kubebuilder:object:root=true

// MemcachedList contains a list of Memcached
type MemcachedList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Memcached `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Memcached{}, &MemcachedList{})
}
