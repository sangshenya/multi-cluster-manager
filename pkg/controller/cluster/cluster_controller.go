package controller

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	"harmonycloud.cn/stellaris/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
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
	r.log.V(4).Info("start reconcile cluster")
	defer r.log.V(4).Info("end reconcile cluster")

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

	// auto deploy proxy
	if cluster.Annotations[AutoDeployProxyAnnotationKey] == AutoDeployProxyAnnotationValue &&
		(cluster.Status.Status == "" || cluster.Status.Status == v1alpha1.InitializingStatus) {
		cluster.Status.Status = v1alpha1.InitializingStatus
		if err := r.Status().Update(ctx, cluster); err != nil {
			r.log.Error(err, "failed update cluster status", "clusterName", cluster.Name)
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
		}
		if err := r.deployProxy(ctx, cluster); err != nil {
			r.log.Error(err, "failed deploy proxy to target cluster", "clusterName", cluster.Name)
			r.Recorder.Event(cluster, "Warning", "FailedDeployProxy", fmt.Sprintf("failed deploy proxy to target cluster: %s: %s", cluster.Spec.ApiServer, err))
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
