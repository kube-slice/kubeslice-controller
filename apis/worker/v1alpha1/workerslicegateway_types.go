/*
 * 	Copyright (c) 2022 Avesha, Inc. All rights reserved. # # SPDX-License-Identifier: Apache-2.0
 *
 * 	Licensed under the Apache License, Version 2.0 (the "License");
 * 	you may not use this file except in compliance with the License.
 * 	You may obtain a copy of the License at
 *
 * 	http://www.apache.org/licenses/LICENSE-2.0
 *
 * 	Unless required by applicable law or agreed to in writing, software
 * 	distributed under the License is distributed on an "AS IS" BASIS,
 * 	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * 	See the License for the specific language governing permissions and
 * 	limitations under the License.
 */

package v1alpha1

import (
	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// WorkerSliceGatewaySpec defines the desired state of WorkerSliceGateway
type WorkerSliceGatewaySpec struct {
	SliceName string `json:"sliceName,omitempty"`
	//+kubebuilder:default:=OpenVPN
	GatewayType controllerv1alpha1.SliceGatewayType `json:"gatewayType,omitempty"`
	//+kubebuilder:validation:Enum:=Client;Server
	GatewayHostType string `json:"gatewayHostType,omitempty"`
	//+kubebuilder:default:=NodePort
	//+kubebuilder:validation:Enum:=NodePort;LoadBalancer
	GatewayConnectivityType string `json:"gatewayConnectivityType,omitempty"`
	//+kubebuilder:default:=UDP
	//+kubebuilder:validation:Enum:=TCP;UDP
	GatewayProtocol     string             `json:"gatewayProtocol,omitempty"`
	GatewayCredentials  GatewayCredentials `json:"gatewayCredentials,omitempty"`
	LocalGatewayConfig  SliceGatewayConfig `json:"localGatewayConfig,omitempty"`
	RemoteGatewayConfig SliceGatewayConfig `json:"remoteGatewayConfig,omitempty"`
	GatewayNumber       int                `json:"gatewayNumber,omitempty"`
}

type SliceGatewayConfig struct {
	//+kubebuilder:deprecatedversion:warning="worker/v1alpha1 NodeIp is deprecated...use NodeIps"
	NodeIp          string   `json:"nodeIp,omitempty"`
	NodeIps         []string `json:"nodeIps,omitempty"`
	LoadBalancerIps []string `json:"loadBalancerIps,omitempty"`
	NodePort        int      `json:"nodePort,omitempty"`
	NodePorts       []int    `json:"nodePorts,omitempty"`
	GatewayName     string   `json:"gatewayName,omitempty"`
	ClusterName     string   `json:"clusterName,omitempty"`
	VpnIp           string   `json:"vpnIp,omitempty"`
	GatewaySubnet   string   `json:"gatewaySubnet,omitempty"`
}

type GatewayCredentials struct {
	SecretName string `json:"secretName,omitempty"`
}

// WorkerSliceGatewayStatus defines the observed state of WorkerSliceGateway
type WorkerSliceGatewayStatus struct {
	GatewayNumber         int `json:"gatewayNumber,omitempty"`
	ClusterInsertionIndex int `json:"clusterInsertionIndex,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// WorkerSliceGateway is the Schema for the slicegateways API
type WorkerSliceGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkerSliceGatewaySpec   `json:"spec,omitempty"`
	Status WorkerSliceGatewayStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WorkerSliceGatewayList contains a list of WorkerSliceGateway
type WorkerSliceGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkerSliceGateway `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WorkerSliceGateway{}, &WorkerSliceGatewayList{})
}
