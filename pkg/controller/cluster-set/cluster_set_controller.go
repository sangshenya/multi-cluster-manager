package cluster_set

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type ClusterSetReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	log    logr.Logger
}

func (r *ClusterSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.log.V(4).Info(fmt.Sprintf("Start Reconciling ClusterSet(%s:%s)", req.Namespace, req.Name))
	defer r.log.V(4).Info(fmt.Sprintf("End Reconciling ClusterSet(%s:%s)", req.Namespace, req.Name))

	clusterSet := &v1alpha1.ClusterSet{}
	if err := r.Client.Get(context.TODO(), req.NamespacedName, clusterSet); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{Requeue: true}, err
	}
	return ctrl.Result{}, nil
}

func (r *ClusterSetReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ClusterSet{}).
		Complete(r)
}

func Setup(mgr ctrl.Manager, controllerCommon controllerCommon.Args) error {
	reconciler := ClusterSetReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		log:    logf.Log.WithName("cluster_set_controller"),
	}
	return reconciler.SetupWithManager(mgr)
}
