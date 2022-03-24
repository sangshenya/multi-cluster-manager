package resource_binding

import (
	"context"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	common "harmonycloud.cn/stellaris/pkg/controller/common"
	"harmonycloud.cn/stellaris/pkg/utils/slice"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Override struct {
	Namespace string
	ClusterName string
	ResourceName string
}

// ApplyResourceOverride overrides the resource if one or more MultiClusterResourceOverride
// exist and match the target resource and target cluster
func ApplyResourceOverride (c client.Client, resource *runtime.RawExtension, Override *Override) (*runtime.RawExtension, error) {
	var err error

	// get override list
	overrideList := &v1alpha1.MultiClusterResourceOverrideList{}
	if err = c.List(context.Background(), overrideList, &client.ListOptions{Namespace: Override.Namespace}); err != nil {
		return resource, err
	}
	if len(overrideList.Items) == 0 {
		return resource, nil
	}

	// match the resource and cluster
	for _, override := range overrideList.Items {
		if !IsExistResource(Override.ResourceName, override.Spec.Resources){
			continue
		}
		for _, cluster := range override.Spec.Clusters {
			if !IsExistCluster(Override.ClusterName, &cluster) {
				continue
			}
			if resource, err = common.ApplyJsonPatch(resource, cluster.Overrides); err != nil {
				klog.Error("Failed to build patches ", err)
			}
		}
	}
	return resource, nil
}

func IsExistResource (resourceName string, resources *v1alpha1.MultiClusterResourceOverrideResources) bool {
	return slice.ContainsString(resources.Names, resourceName)
}

func IsExistCluster (clusterName string, clusters *v1alpha1.MultiClusterResourceOverrideClusters) bool {
	return slice.ContainsString(clusters.Names, clusterName)
}

// NewOverridePolicyFunc watches the resourceOverride
func NewOverridePolicyFunc(c client.Client) handler.MapFunc {
	return func(object client.Object) []reconcile.Request {
		resourceOverride, ok := object.(*v1alpha1.MultiClusterResourceOverride)
		if !ok {
			klog.Error("Failed to transform runtime Object to v1alpha1.MultiClusterResourceOverride")
			return nil
		}
		requestQueue := make([]reconcile.Request, 0)

		bindingList := &v1alpha1.MultiClusterResourceBindingList{}
		err := c.List(context.Background(), bindingList, &client.ListOptions{Namespace: resourceOverride.Namespace})
		if err != nil {
			klog.Error("Failed to get MultiClusterResourceBindingList")
			return nil
		}
		if len(bindingList.Items) == 0 {
			return nil
		}

		// matches clusterName and resourceName
		flag := false
		for _, binding := range bindingList.Items {
			for _, bindingResource := range binding.Spec.Resources {
				if !IsExistResource(bindingResource.Name, resourceOverride.Spec.Resources) {
					continue
				}
				for _, bindingCluster := range bindingResource.Clusters {
					for _, overrideCluster := range resourceOverride.Spec.Clusters {
						if IsExistCluster(bindingCluster.Name, &overrideCluster) {
							flag = true
							break
						}
					}
					if flag {
						break
					}
				}
				if flag {
					requestQueue = append(requestQueue, reconcile.Request{NamespacedName:
						types.NamespacedName{Namespace: binding.Namespace, Name: binding.Name}})
					break
				}
			}
		}
		return requestQueue
	}
}

