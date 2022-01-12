package helper

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

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
