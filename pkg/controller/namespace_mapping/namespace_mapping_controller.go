package namespace_mapping

import (
	"context"
	"github.com/go-logr/logr"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	"harmonycloud.cn/stellaris/pkg/util/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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
	namespaceMapping := &v1alpha1.NamespaceMapping{}
	if err := r.Client.Get(context.TODO(), req.NamespacedName, namespaceMapping); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{Requeue: true}, err
	}
	// remove
	if !namespaceMapping.DeletionTimestamp.IsZero() {
		return r.removeNamespaceMapping(namespaceMapping)
	}
	return r.syncNamespaceMapping(namespaceMapping)
}

func (r *NamespaceMappingReconciler) syncNamespaceMapping(namespaceMapping *v1alpha1.NamespaceMapping) (ctrl.Result, error) {
	// create mapping for cluster
	if err := r.createMapping(namespaceMapping); err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	return r.ensureFinalizer(namespaceMapping)
}

func (r *NamespaceMappingReconciler) createMapping(namespaceMapping *v1alpha1.NamespaceMapping) error {
	err := r.mappingOperator(namespaceMapping, createOrUpdateMapping)
	if err != nil {
		return err
	}
	return nil
}

func (r *NamespaceMappingReconciler) removeNamespaceMapping(namespaceMapping *v1alpha1.NamespaceMapping) (ctrl.Result, error) {
	err := r.removeMapping(namespaceMapping)
	if err != nil {
		klog.Errorf("failed to remove namespaceMapping %s, %v", namespaceMapping.Name, err)
		return ctrl.Result{Requeue: true}, err
	}
	return r.removeFinalizer(namespaceMapping)

}

func (r *NamespaceMappingReconciler) removeMapping(namespaceMapping *v1alpha1.NamespaceMapping) error {
	err := r.mappingOperator(namespaceMapping, removeMapping)
	if err != nil {
		return err
	}
	return nil
}

func (r *NamespaceMappingReconciler) ensureFinalizer(namespaceMapping *v1alpha1.NamespaceMapping) (ctrl.Result, error) {
	if controllerutil.ContainsFinalizer(namespaceMapping, managerCommon.NamespaceMappingControllerFinalizer) {
		return ctrl.Result{}, nil
	}
	controllerutil.AddFinalizer(namespaceMapping, managerCommon.NamespaceMappingControllerFinalizer)
	if err := r.Client.Update(context.TODO(), namespaceMapping); err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{}, nil
}

func (r *NamespaceMappingReconciler) removeFinalizer(namespaceMapping *v1alpha1.NamespaceMapping) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(namespaceMapping, managerCommon.NamespaceMappingControllerFinalizer) {
		return ctrl.Result{}, nil
	}
	controllerutil.RemoveFinalizer(namespaceMapping, managerCommon.NamespaceMappingControllerFinalizer)

	if err := r.Client.Update(context.TODO(), namespaceMapping); err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{}, nil
}

func (r *NamespaceMappingReconciler) mappingOperator(namespaceMapping *v1alpha1.NamespaceMapping, option string) error {
	mappingRule := namespaceMapping.Spec.Mapping
	for ruleK, ruleV := range mappingRule {
		workspace := &corev1.Namespace{}
		r.Client.Get(context.TODO(), types.NamespacedName{Name: namespaceMapping.Namespace}, workspace)
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
			r.Client.Update(context.TODO(), workspace)
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
			r.Client.Update(context.TODO(), workspace)
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
