package helper

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

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
