package controller

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	utils "harmonycloud.cn/stellaris/pkg/utils"
	"harmonycloud.cn/stellaris/pkg/utils/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
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
	r.log.WithValues("Request.Name", req.Name)
	r.log.Info("Reconciling Cluster")

	cluster := &v1alpha1.Cluster{}
	err := r.Client.Get(ctx, req.NamespacedName, cluster)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// add Finalizers
	if controllerCommon.ShouldAddFinalizer(cluster) {
		return r.addFinalizer(ctx, cluster)
	}

	// remove cluster
	if !cluster.DeletionTimestamp.IsZero() {
		return r.removeCluster(ctx, cluster)
	}

	// auto deploy proxy
	if cluster.Annotations[AutoDeployProxyAnnotationKey] == AutoDeployProxyAnnotationValue &&
		(cluster.Status.Status == "" || cluster.Status.Status == v1alpha1.InitializingStatus) {
		cluster.Status.Status = v1alpha1.InitializingStatus
		if err := r.Status().Update(ctx, cluster); err != nil {
			return controllerCommon.ReQueueResult(err)
		}
		if err := r.deployProxy(ctx, cluster); err != nil {
			r.Recorder.Event(cluster, "Warning", "FailedDeployProxy", fmt.Sprintf("failed deploy proxy to target cluster: %s: %s", cluster.Spec.ApiServer, err))
			return controllerCommon.ReQueueResult(err)
		}
	}

	// create workspace for cluster
	if err := r.createWorkspace(ctx, cluster); err != nil {
		return controllerCommon.ReQueueResult(err)
	}

	return ctrl.Result{}, nil
}

// create workspace(namespace) in control plane
func (r *ClusterReconciler) createWorkspace(ctx context.Context, cluster *v1alpha1.Cluster) error {
	clusterWorkspaceName, err := common.GenerateName(managerCommon.ClusterWorkspacePrefix, cluster.Name)
	if err != nil {
		r.log.Error(err, "failed to generate workspace for cluster: %s", cluster.Name)
		return err
	}
	clusterWorkspace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterWorkspaceName,
		},
	}
	err = r.Client.Create(ctx, clusterWorkspace)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			r.log.Error(err, "failed to create workspace for cluster %s", cluster.Name)
			return err
		}
	}
	r.log.Info("Created workspace %s for cluster %s", clusterWorkspaceName, cluster.Name)
	return nil
}

// remove cluster in control plane
func (r *ClusterReconciler) removeCluster(ctx context.Context, cluster *v1alpha1.Cluster) (ctrl.Result, error) {
	err := r.removeClusterInControlPlane(ctx, cluster)
	if errors.IsNotFound(err) {
		return r.removeFinalizer(ctx, cluster)
	}
	if err != nil {
		klog.Errorf("failed to remove workspace %s, %v", cluster.Name, err)
		return controllerCommon.ReQueueResult(err)
	}
	exist, err := r.workspaceExist(ctx, cluster.Name)
	if err != nil {
		klog.Errorf("failed to check if the workspace exist for cluster: %v", err)
		return controllerCommon.ReQueueResult(err)
	} else if exist {
		return ctrl.Result{Requeue: true}, fmt.Errorf("workspace %s still exists,prepare to delete", cluster.Name)
	}
	return r.removeFinalizer(ctx, cluster)

}

// remove workspace(namespace) in control plane
func (r *ClusterReconciler) removeClusterInControlPlane(ctx context.Context, cluster *v1alpha1.Cluster) error {
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
	if err := r.Client.Delete(ctx, clusterWorkspace); err != nil && !errors.IsNotFound(err) {
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

// sync clusterResource Finalizer
func (r *ClusterReconciler) addFinalizer(ctx context.Context, instance *v1alpha1.Cluster) (ctrl.Result, error) {
	err := controllerCommon.AddFinalizer(ctx, r.Client, instance)
	if err != nil {
		r.log.Error(err, fmt.Sprintf("append finalizer failed, resource(%s)", instance.Name))
		return controllerCommon.ReQueueResult(err)
	}
	return ctrl.Result{}, nil
}

// check namespace before remove
func (r *ClusterReconciler) workspaceExist(ctx context.Context, cluster string) (bool, error) {
	clusterWorkspaceName, err := common.GenerateName(managerCommon.ClusterWorkspacePrefix, cluster)
	if err != nil {
		klog.Errorf("failed to generate workspace for cluster %s, %v", cluster, err)
		return false, err
	}
	clusterWorkspaceExist := &corev1.Namespace{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: clusterWorkspaceName}, clusterWorkspaceExist)

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

// kubeconfigGetterForSecret will get kubeconfig from secret in manager cluster
func (r *ClusterReconciler) kubeconfigGetterForStellarisCluster(cluster *v1alpha1.Cluster) clientcmd.KubeconfigGetter {
	return func() (*api.Config, error) {
		secret := &corev1.Secret{}
		if err := r.Get(context.TODO(), types.NamespacedName{Namespace: cluster.Spec.SecretRef.Namespace, Name: cluster.Spec.SecretRef.Name}, secret); err != nil {
			return nil, err
		}
		data, ok := secret.Data[cluster.Spec.SecretRef.Field]
		if !ok {
			return nil, fmt.Errorf("secret %s/%s does not have data with field: %s", cluster.Spec.SecretRef.Name, cluster.Spec.SecretRef.Namespace, cluster.Spec.SecretRef.Field)
		}
		switch cluster.Spec.SecretRef.Type {
		case v1alpha1.KubeConfigType:
			return clientcmd.Load(data)
		case v1alpha1.TokenType:
			clusters := make(map[string]*api.Cluster)
			clusters["kubernetes-cluster"] = &api.Cluster{
				Server:                cluster.Spec.ApiServer,
				InsecureSkipTLSVerify: true,
			}
			contexts := make(map[string]*api.Context)
			contexts["kubernetes-context"] = &api.Context{
				Cluster:  "kubernetes-cluster",
				AuthInfo: "token-auth",
			}
			authInfos := make(map[string]*api.AuthInfo)
			authInfos["token-auth"] = &api.AuthInfo{
				Token: string(data),
			}
			return &api.Config{
				APIVersion:     "v1",
				Kind:           "Config",
				Clusters:       clusters,
				Contexts:       contexts,
				AuthInfos:      authInfos,
				CurrentContext: "kubernetes-context",
			}, nil
		default:
			return nil, fmt.Errorf("secret type must in [%s, %s]", v1alpha1.KubeConfigType, v1alpha1.TokenType)
		}
	}
}

func (r *ClusterReconciler) deployProxy(ctx context.Context, cluster *v1alpha1.Cluster) error {
	cfg, err := clientcmd.BuildConfigFromKubeconfigGetter(cluster.Spec.ApiServer, r.kubeconfigGetterForStellarisCluster(cluster))
	if err != nil {
		return err
	}
	c, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return err
	}

	cm := &corev1.ConfigMap{}
	if err := r.Get(ctx, r.tmplNamespacedName, cm); err != nil {
		return err
	}

	clusterTemplate, exist := cm.Data[AutoDeployCueTemplateField]
	if !exist {
		return fmt.Errorf("cannot find cluster template in ConfigMap %s field %s", r.tmplNamespacedName, AutoDeployCueTemplateField)
	}

	proxyBuilder, err := NewProxyBuilder(cluster)
	if err != nil {
		return err
	}
	resources, err := proxyBuilder.GenerateProxyResources(clusterTemplate)
	if err != nil {
		return err
	}

	// create namespace first
	namespace := proxyBuilder.GenerateNamespaces()
	err = r.Client.Create(ctx, namespace)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	// create proxy addons configmap
	proxyAddonsCfg, err := proxyBuilder.GenerateAddonsConfigMap()
	if err != nil {
		return err
	}
	err = r.Client.Create(ctx, proxyAddonsCfg)
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	// create proxy all resources
	for _, resource := range resources {
		if _, err := c.Resource(utils.GroupVersionResourceFromUnstructured(resource)).Namespace(resource.GetNamespace()).Create(ctx, resource, metav1.CreateOptions{}); err != nil {
			if errors.IsAlreadyExists(err) {
				continue
			}
			return err
		}
	}

	return nil
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
