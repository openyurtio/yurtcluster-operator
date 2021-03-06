/*
Copyright 2021 The OpenYurt Authors.

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

package yurthub

import (
	_ "embed"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// data keys
	StaticCloudPod        = "yurt-hub-static-cloud-pod"
	StaticEdgePod         = "yurt-hub-static-edge-pod"
	HubConfig             = "yurt-hub-config"
	HubClusterRole        = "yurt-hub-cluster-role"
	HubClusterRoleBinding = "yurt-hub-cluster-role-binding"
	YurtKubeletConfig     = "yurt-kubelet-config"
	ContainerName         = "yurt-hub"
)

var (
	//go:embed yurthub.yaml
	TemplateContent string

	NamespacedName = types.NamespacedName{
		Namespace: metav1.NamespaceSystem,
		Name:      "yurt-operator-yurt-hub-template",
	}
)
