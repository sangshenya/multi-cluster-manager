package common

import (
	"context"
	"errors"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	"k8s.io/apimachinery/pkg/types"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/common"

	ctrl "sigs.k8s.io/controller-runtime"

	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	sliceutil "harmonycloud.cn/stellaris/pkg/util/slice"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ShouldAddFinalizer(object client.Object) bool {
	if !sliceutil.ContainsString(object.GetFinalizers(), managerCommon.FinalizerName) && object.GetDeletionTimestamp().IsZero() {
		return true
	}
	return false
}

// AddFinalizers add multi finalizer, do nothing when the multi finalizer is already exists
func AddFinalizer(ctx context.Context, clientSet client.Client, object client.Object) error {
	if sliceutil.ContainsString(object.GetFinalizers(), managerCommon.FinalizerName) {
		return nil
	}
	object.SetFinalizers(append(object.GetFinalizers(), managerCommon.FinalizerName))
	return clientSet.Update(ctx, object)
}

// RemoveFinalizer remove multi finalizer, do nothing when the multi finalizer isn`t already exists
func RemoveFinalizer(ctx context.Context, clientSet client.Client, object client.Object) error {
	if !sliceutil.ContainsString(object.GetFinalizers(), managerCommon.FinalizerName) {
		return nil
	}
	object.SetFinalizers(sliceutil.RemoveString(object.GetFinalizers(), managerCommon.FinalizerName))
	return clientSet.Update(ctx, object)
}

func ReQueueResult(err error) (ctrl.Result, error) {
	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: 30 * time.Second,
	}, err
}

func GetClusterNamespaces(ctx context.Context, clientSet client.Client, sourceType common.ClusterType, clusterNames []string, clusterSetName string) ([]string, error) {
	var clusterNamespaces []string
	switch sourceType {
	case common.ClusterTypeClusters:
		if len(clusterNames) == 0 {
			return clusterNamespaces, errors.New("clusterNames is empty")
		}
		for _, item := range clusterNames {
			clusterNamespaces = append(clusterNamespaces, managerCommon.ClusterNamespace(item))
		}

	case common.ClusterTypeClusterSet:
		if len(clusterSetName) == 0 {
			return clusterNamespaces, errors.New("clusterSetName is empty")
		}
		namespaces, err := getClustersNameSpaceFromClusterSet(ctx, clientSet, clusterSetName)
		if err != nil {
			return clusterNamespaces, err
		}
		clusterNamespaces = namespaces
	}
	return clusterNamespaces, nil
}

func getClustersNameSpaceFromClusterSet(ctx context.Context, clientSet client.Client, clusterSetName string) ([]string, error) {
	clusterSet := &v1alpha1.ClusterSet{}
	err := clientSet.Get(ctx, types.NamespacedName{
		Name: clusterSetName,
	}, clusterSet)
	if err != nil {
		return nil, err
	}
	if len(clusterSet.Spec.Clusters) == 0 && (clusterSet.Spec.Selector.Labels == nil || len(clusterSet.Spec.Selector.Labels) == 0) {
		return nil, errors.New(fmt.Sprintf("clusterSet(%s) is empty", clusterSetName))
	}
	var clusterNamespaces []string
	if len(clusterSet.Spec.Clusters) > 0 {
		for _, item := range clusterSet.Spec.Clusters {
			clusterNamespaces = append(clusterNamespaces, managerCommon.ClusterNamespace(item.Name))
		}
		return clusterNamespaces, nil
	}

	clusterList := &v1alpha1.ClusterList{}
	err = clientSet.List(ctx, clusterList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(clusterSet.Spec.Selector.Labels),
	})
	if err != nil {
		return nil, err
	}
	for _, item := range clusterList.Items {
		clusterNamespaces = append(clusterNamespaces, managerCommon.ClusterNamespace(item.Name))
	}
	return clusterNamespaces, nil
}

func AllCluster(ctx context.Context, clientSet client.Client) (*v1alpha1.ClusterList, error) {
	clusterList := &v1alpha1.ClusterList{}
	err := clientSet.List(ctx, clusterList)
	if err != nil {
		return clusterList, err
	}
	return clusterList, nil
}
