package utils

import (
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/parser"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
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
