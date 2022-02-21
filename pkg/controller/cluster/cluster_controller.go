package controller

import (
	"context"
	"fmt"
	clusterHealth "harmonycloud.cn/stellaris/pkg/common/cluster-health"

	"github.com/go-logr/logr"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	"harmonycloud.cn/stellaris/pkg/util/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type ClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	log    logr.Logger
}

func (r *ClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.WithValues("Request.Name", req.Name)
	r.log.Info("Reconciling Cluster")
	instance := &v1alpha1.Cluster{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// add Finalizers
	if controllerCommon.ShouldAddFinalizer(instance) {
		return r.addFinalizer(ctx, instance)
	}
	// remove cluster
	if !instance.DeletionTimestamp.IsZero() {
		return r.removeCluster(ctx, instance)
	}
	return r.syncCluster(instance)
}

func (r *ClusterReconciler) syncCluster(cluster *v1alpha1.Cluster) (ctrl.Result, error) {
	// create workspace for cluster
	if err := r.createWorkspace(cluster); err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{}, nil
}

func (r *ClusterReconciler) createWorkspace(cluster *v1alpha1.Cluster) error {
	clusterWorkspaceName, err := common.GenerateName(managerCommon.ClusterWorkspacePrefix, cluster.Name)
	if err != nil {
		r.log.Error(err, "failed to generate workspace for cluster: %s", cluster.Name)
		return err
	}
	clusterWorkspaceExist := &corev1.Namespace{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: clusterWorkspaceName}, clusterWorkspaceExist); err != nil {
		if !errors.IsNotFound(err) {
			r.log.Error(err, "failed to get namespace %s", clusterWorkspaceName)
		}
		clusterWorkspace := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterWorkspaceName,
			},
		}
		err := r.Client.Create(context.TODO(), clusterWorkspace)
		if err != nil {
			r.log.Error(err, "failed to create workspace for cluster %s", cluster.Name)
			return err
		}
		r.log.Info("Created workspace %s for cluster %s", clusterWorkspaceName, cluster.Name)
	}
	return nil

}

func (r *ClusterReconciler) removeCluster(ctx context.Context, cluster *v1alpha1.Cluster) (ctrl.Result, error) {
	err := r.removeClusterInControlPlane(cluster)
	if errors.IsNotFound(err) {
		return r.removeFinalizer(ctx, cluster)
	}
	if err != nil {
		klog.Errorf("failed to remove workspace %s, %v", cluster.Name, err)
		return ctrl.Result{Requeue: true}, err
	}
	exist, err := r.workspaceExist(cluster.Name)
	if err != nil {
		klog.Errorf("failed to check if the workspace exist for cluster: %v", err)
		return ctrl.Result{Requeue: true}, err
	} else if exist {
		return ctrl.Result{Requeue: true}, fmt.Errorf("workspace %s still exists,prepare to delete", cluster.Name)
	}
	return r.removeFinalizer(ctx, cluster)

}

func (r *ClusterReconciler) removeClusterInControlPlane(cluster *v1alpha1.Cluster) error {
	clusterWorkspaceName, err := common.GenerateName(managerCommon.ClusterWorkspacePrefix, cluster.Name)
	if err != nil {
		klog.Errorf("failed to generate workspace for cluster %s, %v", cluster.Name, err)
		return err
	}
	clusterWorkspace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterWorkspaceName,
		},
	}
	if err := r.Client.Delete(context.TODO(), clusterWorkspace); err != nil && !errors.IsNotFound(err) {
		klog.Errorf("error while deleting namespace %s: %v", cluster.Name, err)
		return err
	}
	return nil
}

func (r *ClusterReconciler) removeFinalizer(ctx context.Context, instance *v1alpha1.Cluster) (ctrl.Result, error) {
	err := controllerCommon.RemoveFinalizer(ctx, r.Client, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("delete finalizer filed from resource(%s) failed", instance.Name))
		return controllerCommon.ReQueueResult(err)
	}
	return ctrl.Result{}, nil
}

func (r *ClusterReconciler) addCreateStatus(cluster *v1alpha1.Cluster) (ctrl.Result, error) {
	nowTime := metav1.Now()
	createStatus := v1alpha1.ClusterStatus{
		Conditions:                    clusterHealth.GenerateReadyCondition(true, true),
		LastReceiveHeartBeatTimestamp: nowTime,
		LastUpdateTimestamp:           nowTime,
		Healthy:                       true,
		Status:                        v1alpha1.OnlineStatus,
	}
	cluster.Status = createStatus
	if err := r.Status().Update(context.TODO(), cluster); err != nil {
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{}, nil
}

// sync clusterResource Finalizer
func (r *ClusterReconciler) addFinalizer(ctx context.Context, instance *v1alpha1.Cluster) (ctrl.Result, error) {
	err := controllerCommon.AddFinalizer(ctx, r.Client, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("append finalizer failed, resource(%s)", instance.Name))
		return controllerCommon.ReQueueResult(err)
	}
	if instance.Status.LastReceiveHeartBeatTimestamp.IsZero() {
		return r.addCreateStatus(instance)
	}
	return ctrl.Result{}, nil
}

func (r *ClusterReconciler) workspaceExist(cluster string) (bool, error) {
	clusterWorkspaceName, err := common.GenerateName(managerCommon.ClusterWorkspacePrefix, cluster)
	if err != nil {
		klog.Errorf("failed to generate workspace for cluster %s, %v", cluster, err)
		return false, err
	}
	clusterWorkspaceExist := &corev1.Namespace{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: clusterWorkspaceName}, clusterWorkspaceExist)

	if errors.IsNotFound(err) {
		klog.V(2).Infof("workspace for cluster %s not exists: %v", cluster, err)
		return false, nil
	}
	if err != nil {
		klog.Errorf("failed to get workspace for cluster %s: %v", cluster, err)
		return false, nil
	}
	if clusterWorkspaceExist.Status.Phase == corev1.NamespaceTerminating {
		klog.V(2).Infof("workspace for cluster %s is Terminating", cluster)
		return false, nil
	}

	return true, nil
}

func (r *ClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.Cluster{}).
		Complete(r)
}

func Setup(mgr ctrl.Manager, controllerCommon controllerCommon.Args) error {
	reconciler := ClusterReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		log:    logf.Log.WithName("cluster_controller"),
	}
	return reconciler.SetupWithManager(mgr)
}
