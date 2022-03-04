package namespace_mapping

import (
	"context"
	"fmt"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/client-go/tools/record"

	"github.com/go-logr/logr"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	createOrUpdateMapping = "create"
	removeMapping         = "remove"
)

type NamespaceMappingReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	log      logr.Logger
	Recorder record.EventRecorder
}

func (r *NamespaceMappingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.V(4).Info(fmt.Sprintf("Start Reconciling NamespaceMapping(%s:%s)", req.Namespace, req.Name))
	defer r.log.V(4).Info(fmt.Sprintf("End Reconciling NamespaceMapping(%s:%s)", req.Namespace, req.Name))

	instance := &v1alpha1.NamespaceMapping{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// add Finalizers
	if controllerCommon.ShouldAddFinalizer(instance) {
		if err = controllerCommon.AddFinalizer(ctx, r.Client, instance); err != nil {
			r.log.Error(err, fmt.Sprintf("append finalizer failed, resource(%s:%s)", instance.Namespace, instance.Name))
			r.Recorder.Event(instance, "Warning", "FailedAddFinalizers", err.Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
		}
		return ctrl.Result{}, nil
	}
	// remove
	if !instance.DeletionTimestamp.IsZero() {
		if err = r.mappingOperator(ctx, instance, removeMapping); err != nil {
			r.log.Error(err, fmt.Sprintf("delete finalizer plan failed, resource(%s:%s)", instance.Namespace, instance.Name))
			r.Recorder.Event(instance, "Warning", "FailedDeleteFinalizersPlan", err.Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
		}
		if err = controllerCommon.RemoveFinalizer(ctx, r.Client, instance); err != nil {
			r.log.Error(err, fmt.Sprintf("delete finalizer failed, resource(%s:%s)", instance.Namespace, instance.Name))
			r.Recorder.Event(instance, "Warning", "FailedDeleteFinalizers", err.Error())
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
		}
		return ctrl.Result{}, nil
	}
	if err = r.mappingOperator(ctx, instance, createOrUpdateMapping); err != nil {
		r.log.Error(err, fmt.Sprintf("mapping operator failed, resource(%s:%s)", instance.Namespace, instance.Name))
		return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
	}
	return ctrl.Result{}, nil
}

// create,update or remove namespace mapping with label
func (r *NamespaceMappingReconciler) mappingOperator(ctx context.Context, namespaceMapping *v1alpha1.NamespaceMapping, option string) error {
	mappingRule := namespaceMapping.Spec.Mapping
	for ruleK, ruleV := range mappingRule {
		workspace := &corev1.Namespace{}
		err := r.Client.Get(ctx, types.NamespacedName{Name: namespaceMapping.Namespace}, workspace)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return err
		}
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
			err = r.Client.Update(ctx, workspace)
			if err != nil {
				return err
			}
		} else if option == removeMapping {
			// delete mapping label
			if labels == nil {
				continue
			}
			labelK, err := controllerCommon.GenerateLabelKey(ruleK, namespaceMapping.Namespace)
			if err != nil {
				return err
			}
			if len(labelK) > 0 {
				delete(labels, labelK)
			}

			workspace.SetLabels(labels)
			err = r.Client.Update(ctx, workspace)
			if err != nil {
				return err
			}
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
	reconciler.Recorder = mgr.GetEventRecorderFor("stellaris-core")
	return reconciler.SetupWithManager(mgr)
}

func updateLabel(labels map[string]string, cluster string, ruleV string) map[string]string {
	for k, v := range labels {
		if v == ruleV {
			part := managerCommon.NamespaceMappingLabel + cluster
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
