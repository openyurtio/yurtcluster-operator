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

package templates

import (
	"context"
	"fmt"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	"github.com/openyurtio/yurtcluster-operator/pkg/kclient"
	"github.com/openyurtio/yurtcluster-operator/pkg/util"
	"github.com/pkg/errors"
)

// LoadTemplate returns the yurt template with given namespace/name
func LoadTemplate(ctx context.Context, namespacedName types.NamespacedName) (*corev1.ConfigMap, error) {
	cm := &corev1.ConfigMap{}
	if err := kclient.CtlClient().Get(ctx, namespacedName, cm); err != nil {
		return nil, err
	}
	return cm, nil
}

// ApplyTemplateWithExtraArgs renders and merges extra args then applies the manifests into cluster.
func ApplyTemplateWithExtraArgs(ctx context.Context, template *corev1.ConfigMap, dataKey, containerName string, values, extraArgs map[string]string) error {
	yamlContent, ok := template.Data[dataKey]
	if !ok {
		return errors.Errorf("cannot find key %q in manifests template %q", dataKey, klog.KObj(template))
	}
	yamlContentRendered, err := util.RenderTemplate(yamlContent, values)
	if err != nil {
		return errors.Wrapf(err, "failed to render yaml content of %q in manifests template %q", dataKey, klog.KObj(template))
	}
	yamlContentCompleted, err := MergeExtraArgs(yamlContentRendered, extraArgs, containerName)
	if err != nil {
		return errors.Wrapf(err, "failed to merge extra args for %v", containerName)
	}
	if err := util.Apply(ctx, yamlContentCompleted); err != nil {
		return errors.Wrapf(err, "failed to apply yaml content of %q in manifests template %q", dataKey, klog.KObj(template))
	}
	return nil
}

// MergeExtraArgs merges extra args for the target container which defined in the yaml content
func MergeExtraArgs(yamlContent string, extraArgs map[string]string, containerName string) (string, error) {
	obj, err := util.YamlToObject([]byte(yamlContent))
	if err != nil {
		return yamlContent, err
	}

	switch o := obj.(type) {
	case *corev1.Pod:
		return MergeExtraArgsForPod(o, containerName, extraArgs)
	case *appsv1.Deployment:
		return MergeExtraArgsForDeployment(o, containerName, extraArgs)
	case *appsv1.DaemonSet:
		return MergeExtraArgsForDaemonSet(o, containerName, extraArgs)
	default:
		klog.Error("unsupport type %T, skip merge extra args for %v", o, klog.KObj(o))
		return yamlContent, nil
	}
}

// MergeExtraArgsForPod merges the extra args for Pod container and returns the merged yaml content
func MergeExtraArgsForPod(pod *corev1.Pod, containerName string, extraArgs map[string]string) (string, error) {
	for i := range pod.Spec.Containers {
		container := pod.Spec.Containers[i]
		if container.Name == containerName {
			MergeExtraArgsForContainer(&container, extraArgs)
			pod.Spec.Containers[i] = container
		}
	}
	return util.ObjectToYaml(pod)
}

// MergeExtraArgsForDeployment merges the extra args for Deployment container and returns the merged yaml content
func MergeExtraArgsForDeployment(deployment *appsv1.Deployment, containerName string, extraArgs map[string]string) (string, error) {
	for i := range deployment.Spec.Template.Spec.Containers {
		container := deployment.Spec.Template.Spec.Containers[i]
		if container.Name == containerName {
			MergeExtraArgsForContainer(&container, extraArgs)
			deployment.Spec.Template.Spec.Containers[i] = container
		}
	}
	return util.ObjectToYaml(deployment)
}

// MergeExtraArgsForDaemonSet merges the extra args for DaemonSet container and returns the merged yaml content
func MergeExtraArgsForDaemonSet(daemonset *appsv1.DaemonSet, containerName string, extraArgs map[string]string) (string, error) {
	for i := range daemonset.Spec.Template.Spec.Containers {
		container := daemonset.Spec.Template.Spec.Containers[i]
		if container.Name == containerName {
			MergeExtraArgsForContainer(&container, extraArgs)
			daemonset.Spec.Template.Spec.Containers[i] = container
		}
	}
	return util.ObjectToYaml(daemonset)
}

// MergeExtraArgsForContainer merges extraArgs into the given container args
func MergeExtraArgsForContainer(container *corev1.Container, extraArgs map[string]string) {
	var args []string
	argsMap := map[string]string{}
	for _, arg := range container.Args {
		if !strings.HasPrefix(arg, "--") {
			args = append(args, arg)
			continue
		}

		idx := strings.Index(arg, "=")
		if idx == -1 {
			argsMap[arg] = ""
			continue
		}
		argsMap[arg[:idx]] = arg[idx+1:]
	}

	// add or overwrite
	for name, value := range extraArgs {
		// patch -- prefix if not exists
		if !strings.HasPrefix(name, "--") {
			name = fmt.Sprintf("--%s", name)
		}
		argsMap[name] = value
	}

	var keys []string
	for k := range argsMap {
		keys = append(keys, k)
	}

	sort.Strings(keys)
	for _, k := range keys {
		if len(argsMap[k]) > 0 {
			args = append(args, fmt.Sprintf("%s=%s", k, argsMap[k]))
			continue
		}
		args = append(args, k)
	}

	container.Args = args
}
