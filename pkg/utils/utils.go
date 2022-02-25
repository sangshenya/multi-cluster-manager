package utils

import (
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/parser"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	"harmonycloud.cn/stellaris/pkg/model"
	"k8s.io/apimachinery/pkg/api/meta"
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

func ConvertCluster2AddonsModel(cluster v1alpha1.Cluster) model.Plugins {
	addons := model.Plugins{
		InTree: make([]model.In, 0, 1),
	}
	for _, addon := range cluster.Spec.Addons {
		if addon.Type != v1alpha1.InTreeType {
			continue
		}
		// TODO convert configuration to properties
		addonCfg := model.In{
			Name: addon.Name,
		}
		addons.InTree = append(addons.InTree, addonCfg)
	}
	return addons
}
