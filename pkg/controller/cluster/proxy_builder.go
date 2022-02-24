package controller

import (
	"encoding/json"
	"fmt"
	"reflect"

	rawcue "cuelang.org/go/cue"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	"harmonycloud.cn/stellaris/pkg/cue"
	utils "harmonycloud.cn/stellaris/pkg/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

type ProxyBuilder struct {
	Cluster                *v1alpha1.Cluster
	ConfigurationName      string
	ConfigurationNamespace string
}

func NewProxyBuilder(c *v1alpha1.Cluster) (*ProxyBuilder, error) {
	if c.Spec.Configuration == nil {
		return nil, fmt.Errorf("spec.configuration cannot be null")
	}
	cfg := make(map[string]interface{})
	err := json.Unmarshal(c.Spec.Configuration.Raw, &cfg)
	if err != nil {
		return nil, err
	}
	if _, exist := cfg["name"]; !exist {
		return nil, fmt.Errorf("spec.configuration.name cannot be null")
	}
	if _, exist := cfg["namespace"]; !exist {
		return nil, fmt.Errorf("spec.configuration.namespace cannot be null")
	}
	name, ok := cfg["name"].(string)
	if !ok {
		return nil, fmt.Errorf("spec.configuration.name must be string, but got %s", reflect.TypeOf(cfg["name"]))
	}
	namespace, ok := cfg["namespace"].(string)
	if !ok {
		return nil, fmt.Errorf("spec.configuration.namespace must be string, but got %s", reflect.TypeOf(cfg["namespace"]))
	}

	return &ProxyBuilder{Cluster: c, ConfigurationName: name, ConfigurationNamespace: namespace}, nil
}

func (pb *ProxyBuilder) GenerateNamespaces() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: pb.ConfigurationNamespace,
		},
	}
}

// GenerateAddonsConfigMap will use cluster spec generate proxy addons configmap
// TODO support in-tree addon only
func (pb *ProxyBuilder) GenerateAddonsConfigMap() (*corev1.ConfigMap, error) {
	proxyAddonsConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pb.ConfigurationName,
			Namespace: pb.ConfigurationNamespace,
		},
		Data: make(map[string]string),
	}
	addons := utils.ConvertCluster2AddonsModel(*pb.Cluster)
	b, err := yaml.Marshal(addons)
	if err != nil {
		return nil, err
	}
	proxyAddonsConfigMap.Data["addons.yaml"] = string(b)
	return proxyAddonsConfigMap, nil
}

// GenerateProxyResources will use parameters in cluster override cue template and return map[key][unstructured]
func (pb *ProxyBuilder) GenerateProxyResources(clusterTemplate string) (map[string]*unstructured.Unstructured, error) {
	result := make(map[string]*unstructured.Unstructured)

	inst, err := cue.Complete(cue.NewClusterContext(pb.Cluster), clusterTemplate, pb.Cluster.Spec.Configuration)
	if err != nil {
		return nil, err
	}

	outputs := inst.LookupPath(rawcue.ParsePath("outputs"))

	if outputs.Exists() {
		st, err := outputs.Struct()
		if err != nil {
			return nil, err
		}
		for i := 0; i < st.Len(); i++ {
			subValue := st.Field(i).Value
			k, exist := subValue.Label()
			if !exist {
				continue
			}
			v := &unstructured.Unstructured{}
			generated, err := subValue.MarshalJSON()
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(generated, v); err != nil {
				return nil, err
			}
			result[k] = v
		}
	}

	return result, nil
}
