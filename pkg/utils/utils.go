package utils

import (
	"encoding/json"

	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/parser"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	"harmonycloud.cn/stellaris/pkg/model"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func ParseAndAddCueFile(bi *build.Instance, fieldName string, content interface{}) error {
	f, err := parser.ParseFile(fieldName, content, parser.ParseComments)
	if err != nil {
		return err
	}
	if err := bi.AddSyntax(f); err != nil {
		return err
	}
	return nil
}

func GroupVersionResourceFromUnstructured(u *unstructured.Unstructured) schema.GroupVersionResource {
	gvr, _ := meta.UnsafeGuessKindToResource(u.GetObjectKind().GroupVersionKind())
	return gvr
}

func GenerateNamespaceInControlPlane(cluster *v1alpha1.Cluster) *corev1.Namespace {
	namespaceName := managerCommon.ClusterNamespaceInControlPlanePrefix + cluster.Name
	labels := map[string]string{
		managerCommon.ProxyWorkspaceLabelName: cluster.Name,
	}
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   namespaceName,
			Labels: labels,
		},
	}
}

func ConvertCluster2AddonsModel(cluster v1alpha1.Cluster) model.Addons {
	addons := model.Addons{
		InTree: make([]model.In, 0, 1),
	}
	for _, addon := range cluster.Spec.Addons {
		if addon.Type != v1alpha1.InTreeType {
			continue
		}
		addonCfgData, err := addon.Configuration.MarshalJSON()
		if err != nil {
			continue
		}
		inTreeCfg := &model.InTreeConfig{}
		err = json.Unmarshal(addonCfgData, inTreeCfg)
		if err != nil {
			continue
		}
		addons.InTree = append(addons.InTree, model.In{
			Name:           addon.Name,
			Configurations: inTreeCfg,
		})
	}
	return addons
}
