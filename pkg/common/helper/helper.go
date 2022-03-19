package helper

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"k8s.io/apimachinery/pkg/labels"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func GetKubeConfig(masterURL string) (*rest.Config, error) {
	if len(os.Getenv("KUBECONFIG")) > 0 {
		return clientcmd.BuildConfigFromFlags(masterURL, os.Getenv("KUBECONFIG"))
	}
	if c, err := rest.InClusterConfig(); err == nil {
		return c, nil
	}
	if usr, err := user.Current(); err == nil {
		if c, err := clientcmd.BuildConfigFromFlags(
			"",
			filepath.Join(usr.HomeDir, ".kube", "config"),
		); err == nil {
			return c, nil
		}
	}
	return nil, fmt.Errorf("could not locate a kubeconfig")
}

func GetResourceForRawExtension(resource *runtime.RawExtension) (*unstructured.Unstructured, error) {
	resourceByte, err := resource.MarshalJSON()
	if err != nil {
		return nil, err
	}
	resourceObject := &unstructured.Unstructured{}
	err = resourceObject.UnmarshalJSON(resourceByte)
	if err != nil {
		return nil, err
	}
	return resourceObject, nil
}

// GetMappingNamespace Gets the mapping of the namespace in proxy
func GetMappingNamespace(ctx context.Context, clientSet client.Client, proxyClusterName, ns string) (string, error) {
	var namespace *v1.Namespace
	err := clientSet.Get(ctx, types.NamespacedName{
		Name: ns,
	}, namespace)
	if err != nil {
		return "", err
	}

	n, ok := namespace.GetLabels()[proxyClusterName]
	if !ok {
		return ns, nil
	}
	return n, nil
}

// GetNamespaceMapping proxy namespace mapping in core
func GetNamespaceMapping(ctx context.Context, clientSet client.Client, proxyNs, proxyClusterName string) (string, error) {
	nsList := &v1.NamespaceList{}
	set := labels.Set{
		proxyClusterName: proxyNs,
	}
	err := clientSet.List(ctx, nsList, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(set),
	})
	if err != nil {
		return "", err
	}
	if len(nsList.Items) == 0 {
		return proxyNs, nil
	}
	return nsList.Items[0].Name, nil
}
