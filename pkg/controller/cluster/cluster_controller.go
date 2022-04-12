package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	"harmonycloud.cn/stellaris/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	AutoDeployProxyAnnotationKey   = "auto.deploy/stellaris.harmonycloud.cn"
	AutoDeployProxyAnnotationValue = "true"
	AutoDeployCueTemplateField     = "deploy-proxy.cue"
)

type ClusterReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	log                logr.Logger
	Recorder           record.EventRecorder
	tmplNamespacedName types.NamespacedName
}

func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.V(4).Info(fmt.Sprintf("Start Reconciling Cluster(%s:%s)", req.Namespace, req.Name))
	defer r.log.V(4).Info(fmt.Sprintf("End Reconciling Cluster(%s:%s)", req.Namespace, req.Name))

	cluster := &v1alpha1.Cluster{}
	err := r.Client.Get(ctx, req.NamespacedName, cluster)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// create event need add finalizers
	if controllerCommon.ShouldAddFinalizer(cluster) {
		if err := controllerCommon.AddFinalizer(ctx, r.Client, cluster); err != nil {
			r.log.Error(err, "failed add finalizers to cluster", "clusterName", cluster.Name)
			r.Recorder.Event(cluster, "Warning", "FailedAddFinalizers", fmt.Sprintf("failed add finalizers to cluster: %s", err))
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
		}
	}
	// delete event need delete cluster
	if !cluster.DeletionTimestamp.IsZero() {
		if err := r.deleteCluster(ctx, cluster); err != nil {
			r.log.Error(err, "failed delete cluster", "clusterName", cluster.Name)
			r.Recorder.Event(cluster, "Warning", "FailedDeleteCluster", fmt.Sprintf("failed delete cluster: %s", err))
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
		}
	}

	// create namespace in control plane
	if err := r.Client.Create(ctx, utils.GenerateNamespaceInControlPlane(cluster)); err != nil && !errors.IsAlreadyExists(err) {
		r.log.Error(err, "failed create namespace for cluster in control plane", "clusterName", cluster.Name)
		r.Recorder.Event(cluster, "Warning", "FailedCreateNamespace", fmt.Sprintf("failed create namespace for cluster in control plane: %s", err))
		return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
	}

	return ctrl.Result{}, nil
}

// deleteCluster will delete cluster in control plane
func (r *ClusterReconciler) deleteCluster(ctx context.Context, cluster *v1alpha1.Cluster) error {
	// delete namespace in control plane
	if err := r.Client.Delete(ctx, utils.GenerateNamespaceInControlPlane(cluster)); err != nil && !errors.IsNotFound(err) {
		return err
	}
	// remove finalizer
	err := controllerCommon.RemoveFinalizer(ctx, r.Client, cluster)
	if err != nil {
		return err
	}
	return nil
}

func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Cluster{}).
		Complete(r)
}

func Setup(mgr ctrl.Manager, controllerCommon controllerCommon.Args) error {
	reconciler := ClusterReconciler{
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		Recorder:           mgr.GetEventRecorderFor("stellaris-core"),
		tmplNamespacedName: controllerCommon.TmplNamespacedName,
		log:                logf.Log.WithName("cluster_controller"),
	}
	return reconciler.SetupWithManager(mgr)
}
