package namespace_mapping

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	"harmonycloud.cn/stellaris/pkg/util/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
)

const (
	createOrUpdateMapping = "create"
	removeMapping         = "remove"
)

type NamespaceMappingReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	log    logr.Logger
}

func (r *NamespaceMappingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	r.log.Info("Reconciling NamespaceMapping")
	instance := &v1alpha1.NamespaceMapping{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// add Finalizers
	if controllerCommon.ShouldAddFinalizer(instance) {
		return r.addFinalizer(ctx, instance)
	}
	// remove
	if !instance.DeletionTimestamp.IsZero() {
		return r.removeNamespaceMapping(ctx, instance)
	}
	return r.syncNamespaceMapping(ctx, instance)
}

func (r *NamespaceMappingReconciler) syncNamespaceMapping(ctx context.Context, namespaceMapping *v1alpha1.NamespaceMapping) (ctrl.Result, error) {
	// create mapping for cluster
	if err := r.createMapping(ctx, namespaceMapping); err != nil {
		return controllerCommon.ReQueueResult(err)
	}
	return ctrl.Result{}, nil
}

// create namespace mapping
func (r *NamespaceMappingReconciler) createMapping(ctx context.Context, namespaceMapping *v1alpha1.NamespaceMapping) error {
	err := r.mappingOperator(ctx, namespaceMapping, createOrUpdateMapping)
	if err != nil {
		return err
	}
	return nil
}

// remove namespace mapping,then remove finalizer
func (r *NamespaceMappingReconciler) removeNamespaceMapping(ctx context.Context, namespaceMapping *v1alpha1.NamespaceMapping) (ctrl.Result, error) {
	err := r.removeMapping(ctx, namespaceMapping)
	if err != nil {
		klog.Errorf("failed to remove namespaceMapping %s, %v", namespaceMapping.Name, err)
		return controllerCommon.ReQueueResult(err)
	}
	return r.removeFinalizer(ctx, namespaceMapping)

}

// remove namespace mapping
func (r *NamespaceMappingReconciler) removeMapping(ctx context.Context, namespaceMapping *v1alpha1.NamespaceMapping) error {
	err := r.mappingOperator(ctx, namespaceMapping, removeMapping)
	if err != nil {
		return err
	}
	return nil
}

// sync clusterResource Finalizer
func (r *NamespaceMappingReconciler) addFinalizer(ctx context.Context, instance *v1alpha1.NamespaceMapping) (ctrl.Result, error) {
	err := controllerCommon.AddFinalizer(ctx, r.Client, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("append finalizer failed, resource(%s)", instance.Name))
		return controllerCommon.ReQueueResult(err)
	}
	return ctrl.Result{}, nil
}

func (r *NamespaceMappingReconciler) removeFinalizer(ctx context.Context, instance *v1alpha1.NamespaceMapping) (ctrl.Result, error) {
	err := controllerCommon.RemoveFinalizer(ctx, r.Client, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("delete finalizer filed from resource(%s) failed", instance.Name))
		return controllerCommon.ReQueueResult(err)
	}
	return ctrl.Result{}, nil
}

// create,update or remove namespace mapping with label
func (r *NamespaceMappingReconciler) mappingOperator(ctx context.Context, namespaceMapping *v1alpha1.NamespaceMapping, option string) error {
	mappingRule := namespaceMapping.Spec.Mapping
	for ruleK, ruleV := range mappingRule {
		workspace := &corev1.Namespace{}
		r.Client.Get(ctx, types.NamespacedName{Name: namespaceMapping.Namespace}, workspace)
		labels := workspace.GetLabels()
		// add mapping label
		if option == createOrUpdateMapping {
			if labels == nil {
				labels = make(map[string]string, 1)
			} else {
				// if update,delete old labels
				labels = updateLabel(labels, ruleK, ruleV)
			}
			labelK, err := controllerCommon.GenerateLabelKey(ruleK, namespaceMapping.Namespace)
			if err != nil {
				return err
			}
			labels[labelK] = ruleV
			workspace.SetLabels(labels)
			r.Client.Update(ctx, workspace)
		} else if option == removeMapping {
			// delete mapping label
			if labels == nil {
				continue
			}
			labelK, err := controllerCommon.GenerateLabelKey(ruleK, namespaceMapping.Namespace)
			if err != nil {
				return err
			}
			delete(labels, labelK)
			workspace.SetLabels(labels)
			r.Client.Update(ctx, workspace)
		}
	}
	return nil
}

func (r *NamespaceMappingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.NamespaceMapping{}).
		Complete(r)
}

func Setup(mgr ctrl.Manager, controllerCommon controllerCommon.Args) error {
	reconciler := NamespaceMappingReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		log:    logf.Log.WithName("namespace-mapping_controller"),
	}
	return reconciler.SetupWithManager(mgr)
}

func updateLabel(labels map[string]string, cluster string, ruleV string) map[string]string {
	for k, v := range labels {
		if v == ruleV {
			part, _ := common.GenerateName(managerCommon.NamespaceMappingLabel, cluster)
			update := strings.Contains(k, part+"_")
			if update {
				delete(labels, k)
			} else {
				continue
			}
		}
	}
	return labels
}
