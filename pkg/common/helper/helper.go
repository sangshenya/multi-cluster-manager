package helper

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"

	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	v1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	clientConfig "sigs.k8s.io/controller-runtime/pkg/client/config"
)

// ForceRunModeEnv indicates if the operator should be forced to run in either local
// or cluster mode (currently only used for local mode)
var ForceRunModeEnv = "OSDK_FORCE_RUN_MODE"

type RunModeType string

const (
	LocalRunMode   RunModeType = "local"
	ClusterRunMode RunModeType = "cluster"
)

const (
	// OperatorNameEnvVar is the constant for env variable OPERATOR_NAME
	// which is the name of the current operator
	OperatorNameEnvVar = "OPERATOR_NAME"
)

// GetOperatorNamespace returns the namespace the operator should be running in.
// source "github.com/operator-framework/operator-sdk/pkg/k8sutil"
func GetOperatorNamespace() (string, error) {
	if isRunModeLocal() {
		return "stellaris-system", nil
	}
	nsBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.New("namespace not found for current environment")
		}
		return "", err
	}
	ns := strings.TrimSpace(string(nsBytes))
	return ns, nil
}

// GetOperatorName return the operator name
func GetOperatorName() (string, error) {
	operatorName, found := os.LookupEnv(OperatorNameEnvVar)
	if !found {
		return "", fmt.Errorf("%s must be set", OperatorNameEnvVar)
	}
	if len(operatorName) == 0 {
		return "", fmt.Errorf("%s must not be empty", OperatorNameEnvVar)
	}
	return operatorName, nil
}

func isRunModeLocal() bool {
	runmode := os.Getenv(ForceRunModeEnv)
	if len(runmode) == 0 {
		return true
	}
	return runmode == string(LocalRunMode)
}

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

func GetKubeClient() (client.Client, error) {
	restConfig, err := clientConfig.GetConfig()
	if err != nil {
		return nil, err
	}

	clientSet, err := client.New(restConfig, client.Options{})
	if err != nil {
		return nil, err
	}
	return clientSet, nil
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
