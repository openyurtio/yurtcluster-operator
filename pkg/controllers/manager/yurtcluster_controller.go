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

package manager

import (
	"context"
	"fmt"
	"strconv"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	"k8s.io/utils/integer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	operatorv1alpha1 "github.com/openyurtio/yurtcluster-operator/api/v1alpha1"
	controllersutil "github.com/openyurtio/yurtcluster-operator/pkg/controllers/util"
	"github.com/openyurtio/yurtcluster-operator/pkg/patcher"
	"github.com/openyurtio/yurtcluster-operator/pkg/projectinfo"
	"github.com/openyurtio/yurtcluster-operator/pkg/templates"
	"github.com/openyurtio/yurtcluster-operator/pkg/templates/trans"
	"github.com/openyurtio/yurtcluster-operator/pkg/templates/yurthub"
	"github.com/openyurtio/yurtcluster-operator/pkg/templates/yurttunnel"
	"github.com/openyurtio/yurtcluster-operator/pkg/util"
)

// YurtClusterReconciler reconciles a YurtCluster object
type YurtClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme

	recorder record.EventRecorder
}

func (r *YurtClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	// quick return
	if req.Name != operatorv1alpha1.SingletonYurtClusterInstanceName {
		return ctrl.Result{}, nil
	}

	log := r.Log.WithValues("YurtCluster", req.NamespacedName)

	yurtCluster := &operatorv1alpha1.YurtCluster{}
	err := r.Get(ctx, req.NamespacedName, yurtCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get YurtCluster", "Name", req.NamespacedName)
		return ctrl.Result{}, err
	}

	// initialize the patch helper.
	patchHelper, err := patcher.NewHelper(yurtCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		// Always reconcile the Status field.
		if err := r.reconcileStatus(ctx, yurtCluster, reterr); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}

		// Always attempt to Patch the Cluster object and status after each reconciliation.
		// Patch ObservedGeneration only if the reconciliation completed successfully
		patchOpts := []patcher.Option{}
		if reterr == nil {
			patchOpts = append(patchOpts, patcher.WithStatusObservedGeneration{})
		}
		if err := patchHelper.Patch(ctx, yurtCluster, patchOpts...); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// add finalizer if not exist and complete default fields
	if !controllerutil.ContainsFinalizer(yurtCluster, operatorv1alpha1.YurtClusterFinalizer) {
		controllerutil.AddFinalizer(yurtCluster, operatorv1alpha1.YurtClusterFinalizer)
		yurtCluster.Complete()
		return ctrl.Result{}, nil
	}

	// handle deletion reconciliation loop
	if !yurtCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, yurtCluster)
	}

	// handle normal reconciliation loop
	return r.reconcile(ctx, yurtCluster)
}

func (r *YurtClusterReconciler) reconcileDelete(ctx context.Context, yurtCluster *operatorv1alpha1.YurtCluster) (ctrl.Result, error) {
	var errs []error

	if err := r.reconcileYurtHubDelete(ctx); err != nil {
		errs = append(errs, err)
	}

	if err := r.reconcileYurtTunnelDelete(ctx, yurtCluster); err != nil {
		errs = append(errs, err)
	}

	return ctrl.Result{}, kerrors.NewAggregate(errs)
}

func (r *YurtClusterReconciler) reconcile(ctx context.Context, yurtCluster *operatorv1alpha1.YurtCluster) (ctrl.Result, error) {
	var errs []error

	// ensure trans script template
	if err := util.Apply(ctx, trans.TemplateContent); err != nil {
		errs = append(errs, err)
	}

	// ensure yurt hub
	if err := r.reconcileYurtHub(ctx); err != nil {
		errs = append(errs, err)
	}

	// ensure yurt tunnel
	if err := r.reconcileYurtTunnel(ctx, yurtCluster); err != nil {
		errs = append(errs, err)
	}

	return ctrl.Result{}, kerrors.NewAggregate(errs)
}

func (r *YurtClusterReconciler) reconcileYurtHub(ctx context.Context) error {
	if err := r.reconcileYurtHubTemplate(ctx); err != nil {
		return err
	}

	tpl, err := templates.LoadTemplate(ctx, yurthub.NamespacedName)
	if err != nil {
		return errors.Wrap(err, "failed to load yurt hub template")
	}

	keys := []string{
		yurthub.HubConfig,
		yurthub.HubClusterRole,
		yurthub.HubClusterRoleBinding,
	}

	values := map[string]string{}

	// add or update objects
	for _, key := range keys {
		if err := util.ApplyTemplateWithRender(ctx, tpl, key, values); err != nil {
			return err
		}
	}

	return nil
}

func (r *YurtClusterReconciler) reconcileYurtHubTemplate(ctx context.Context) error {
	return util.Apply(ctx, yurthub.TemplateContent)
}

func (r *YurtClusterReconciler) reconcileYurtHubDelete(ctx context.Context) error {
	tpl, err := templates.LoadTemplate(ctx, yurthub.NamespacedName)
	if err != nil {
		return errors.Wrap(err, "failed to load yurt hub template")
	}

	keys := []string{
		yurthub.HubConfig,
		yurthub.HubClusterRole,
		yurthub.HubClusterRoleBinding,
	}

	values := map[string]string{}

	// add or update objects
	for _, key := range keys {
		if err := util.DeleteTemplateWithRender(ctx, tpl, key, values); err != nil {
			return err
		}
	}

	return nil
}

func (r *YurtClusterReconciler) reconcileYurtTunnel(ctx context.Context, yurtCluster *operatorv1alpha1.YurtCluster) error {
	if err := r.reconcileYurtTunnelTemplate(ctx); err != nil {
		return err
	}

	tpl, err := templates.LoadTemplate(ctx, yurttunnel.NamespacedName)
	if err != nil {
		return errors.Wrap(err, "failed to load yurt tunnel template")
	}

	if err := r.reconcileYurtTunnelServer(ctx, tpl, yurtCluster); err != nil {
		return err
	}
	return r.reconcileYurtTunnelAgent(ctx, tpl, yurtCluster)
}

func (r *YurtClusterReconciler) reconcileYurtTunnelTemplate(ctx context.Context) error {
	return util.Apply(ctx, yurttunnel.TemplateContent)
}

func (r *YurtClusterReconciler) reconcileYurtTunnelServer(ctx context.Context, tpl *corev1.ConfigMap, yurtCluster *operatorv1alpha1.YurtCluster) error {
	if !*yurtCluster.Spec.YurtTunnel.Server.Enabled {
		return r.reconcileYurtTunnelServerDelete(ctx, tpl)
	}

	addList := []string{
		yurttunnel.ServerServiceAccount,
		yurttunnel.ServerClusterRole,
		yurttunnel.ServerClusterRoleBinding,
		yurttunnel.ServerService,
		yurttunnel.ServerInternalService,
		yurttunnel.ServerConfigMap,
	}

	var delList []string

	if len(yurtCluster.Spec.YurtTunnel.Server.PublicIP) != 0 {
		addList = append(addList, yurttunnel.ServerNodePortService)
	} else {
		delList = append(delList, yurttunnel.ServerNodePortService)
	}

	controlPlaneSize, err := util.GetControlPlaneSize(ctx, r.Client)
	if err != nil {
		r.Log.Error(err, "failed to count control-plane size")
	}

	values := map[string]string{
		"publicIP":                           yurtCluster.Spec.YurtTunnel.Server.PublicIP,
		"publicPort":                         strconv.Itoa(yurtCluster.Spec.YurtTunnel.Server.PublicPort),
		"edgeNodeLabel":                      projectinfo.GetEdgeWorkerLabelKey(),
		"yurtTunnelServerImage":              util.GetYurtComponentImageByType(yurtCluster, util.YurtTunnelServerImage),
		"yurtTunnelServerCount":              strconv.Itoa(integer.IntMax(controlPlaneSize, yurtCluster.Spec.YurtTunnel.Server.ServerCount)),
		"yurtTunnelServerDeploymentReplicas": strconv.Itoa(integer.IntMax(yurtCluster.Spec.YurtTunnel.Server.ServerCount-controlPlaneSize, 0)),
	}

	// remove objects if need
	for _, key := range delList {
		if err := util.DeleteTemplateWithRender(ctx, tpl, key, values); err != nil {
			return err
		}
	}

	// add or update objects
	for _, key := range addList {
		if err := util.ApplyTemplateWithRender(ctx, tpl, key, values); err != nil {
			return err
		}
	}

	// deal with extra args for DaemonSet and Deployment
	extraArgsComponents := []string{
		yurttunnel.ServerDaemonSet,
		yurttunnel.ServerDeployment,
	}

	for _, key := range extraArgsComponents {
		if err := templates.ApplyTemplateWithExtraArgs(ctx, tpl, key, yurttunnel.ServerContainerName, values, yurtCluster.Spec.YurtTunnel.Server.ExtraArgs); err != nil {
			return err
		}
	}

	return nil
}

func (r *YurtClusterReconciler) reconcileYurtTunnelAgent(ctx context.Context, tpl *corev1.ConfigMap, yurtCluster *operatorv1alpha1.YurtCluster) error {
	if !*yurtCluster.Spec.YurtTunnel.Agent.Enabled {
		return r.reconcileYurtTunnelAgentDelete(ctx, tpl)
	}

	keys := []string{
		yurttunnel.AgentDaemonSet,
	}

	values := map[string]string{
		"edgeNodeLabel":        projectinfo.GetEdgeWorkerLabelKey(),
		"yurtTunnelAgentImage": util.GetYurtComponentImageByType(yurtCluster, util.YurtTunnelAgentImage),
	}

	// normal reconcile
	for _, key := range keys {
		if err := templates.ApplyTemplateWithExtraArgs(ctx, tpl, key, yurttunnel.AgentContainerName, values, yurtCluster.Spec.YurtTunnel.Agent.ExtraArgs); err != nil {
			return err
		}
	}
	return nil
}

func (r *YurtClusterReconciler) reconcileYurtTunnelDelete(ctx context.Context, yurtCluster *operatorv1alpha1.YurtCluster) error {
	tpl, err := templates.LoadTemplate(ctx, yurttunnel.NamespacedName)
	if err != nil {
		return errors.Wrap(err, "failed to load yurt tunnel template")
	}

	if err := r.reconcileYurtTunnelServerDelete(ctx, tpl); err != nil {
		return err
	}

	return r.reconcileYurtTunnelAgentDelete(ctx, tpl)
}

func (r *YurtClusterReconciler) reconcileYurtTunnelServerDelete(ctx context.Context, tpl *corev1.ConfigMap) error {
	delList := []string{
		yurttunnel.ServerDaemonSet,
		yurttunnel.ServerDeployment,
		yurttunnel.ServerService,
		yurttunnel.ServerInternalService,
		yurttunnel.ServerNodePortService,
	}

	dummyValues := map[string]string{
		"publicIP":                           "127.0.0.1",
		"publicPort":                         "0",
		"edgeNodeLabel":                      "label",
		"yurtTunnelServerImage":              "image",
		"yurtTunnelServerCount":              "0",
		"yurtTunnelServerDeploymentReplicas": "0",
	}

	for _, key := range delList {
		if err := util.DeleteTemplateWithRender(ctx, tpl, key, dummyValues); err != nil {
			return err
		}
	}

	return nil
}

func (r *YurtClusterReconciler) reconcileYurtTunnelAgentDelete(ctx context.Context, tpl *corev1.ConfigMap) error {
	delList := []string{
		yurttunnel.AgentDaemonSet,
	}

	dummyValues := map[string]string{
		"edgeNodeLabel":        "label",
		"yurtTunnelAgentImage": "image",
	}

	for _, key := range delList {
		if err := util.DeleteTemplateWithRender(ctx, tpl, key, dummyValues); err != nil {
			return err
		}
	}

	return nil
}

func (r *YurtClusterReconciler) reconcileStatus(ctx context.Context, yurtCluster *operatorv1alpha1.YurtCluster, reterr error) error {
	if reterr == nil {
		yurtCluster.Status.ObservedGeneration = yurtCluster.Generation
	}

	yurtCluster.Status.Phase = operatorv1alpha1.PhaseConverting
	if !yurtCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		yurtCluster.Status.Phase = operatorv1alpha1.PhaseDeleting
	}

	nodeList := &corev1.NodeList{}
	if err := r.Client.List(ctx, nodeList); err != nil {
		return errors.Wrap(err, "failed to list nodes from cluster to check the node convert/revert status")
	}

	for i := range nodeList.Items {
		node := &nodeList.Items[i]
		if !controllersutil.IsNodeConvertOrRevertCompleted(node, yurtCluster) {
			return nil
		}
	}

	if yurtCluster.Status.ObservedGeneration == yurtCluster.Generation {
		// all nodes convert/revert completed and operator apply resources completed
		yurtCluster.Status.Phase = operatorv1alpha1.PhaseSucceed

		// remove finalizer if we can
		if !yurtCluster.ObjectMeta.DeletionTimestamp.IsZero() {
			controllerutil.RemoveFinalizer(yurtCluster, operatorv1alpha1.YurtClusterFinalizer)
		}
	}

	return nil
}

func (r *YurtClusterReconciler) NodeToYurtClusterMapFunc(o client.Object) []ctrl.Request {
	node, ok := o.(*corev1.Node)
	if !ok {
		r.Log.Error(nil, fmt.Sprintf("expected a Node but got a %T", o))
		return nil
	}

	// mark control-plane node as cloud node
	if util.IsControlPlaneNode(node) {
		if err := r.ensureControlPlaneNodeEdgeLabel(context.Background(), node); err != nil {
			r.Log.Error(err, "failed to patch label for node", "Name", klog.KObj(node))
		}
	}

	return []ctrl.Request{
		{
			NamespacedName: types.NamespacedName{Name: operatorv1alpha1.SingletonYurtClusterInstanceName},
		},
	}
}

// ensureControlPlaneNodeEdgeLabel marks master with is-edge-worker/false label if not exists
func (r *YurtClusterReconciler) ensureControlPlaneNodeEdgeLabel(ctx context.Context, node *corev1.Node) error {
	patchHelperNode, err := patcher.NewHelper(node, r.Client)
	if err != nil {
		return err
	}
	node.Labels[projectinfo.GetEdgeWorkerLabelKey()] = "false"
	return patchHelperNode.Patch(ctx, node)
}

func (r *YurtClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	err := ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.YurtCluster{}).
		Watches(
			&source.Kind{Type: &corev1.Node{}},
			handler.EnqueueRequestsFromMapFunc(r.NodeToYurtClusterMapFunc),
		).
		WithOptions(options).
		Complete(r)

	if err != nil {
		return errors.Wrap(err, "failed setting up with a controller manager")
	}

	r.recorder = mgr.GetEventRecorderFor("yurt-cluster-controller")
	return nil
}
