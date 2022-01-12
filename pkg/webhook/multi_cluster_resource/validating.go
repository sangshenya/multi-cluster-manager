package multi_cluster_resource

import (
	"context"
	"net/http"
	"strings"

	managerCommon "harmonycloud.cn/stellaris/pkg/common"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	validationCommon "harmonycloud.cn/stellaris/pkg/common/validation"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ValidatingAdmission validates multiClusterResource object when creating/updating/deleting.
type ValidatingAdmission struct {
	decoder *admission.Decoder
}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	multiClusterResource := &v1alpha1.MultiClusterResource{}
	err := v.decoder.Decode(req, multiClusterResource)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	klog.V(2).Infof("Validating multiClusterResource(%s) for request: %s", multiClusterResource.Name, req.Operation)

	if multiClusterResource.Spec.Resource == nil {
		klog.Error(validationCommon.ResourceIsNil)
		return admission.Denied(validationCommon.ResourceIsNil)
	}

	if multiClusterResource.Spec.ResourceRef == nil {
		klog.Error(validationCommon.ResourceRefIsNil)
		return admission.Denied(validationCommon.ResourceRefIsNil)
	}

	if !strings.HasPrefix(multiClusterResource.GetName(), managerCommon.GvkLabelString(multiClusterResource.Spec.ResourceRef)) {
		klog.Error(validationCommon.NamePrefixedGVK)
		return admission.Denied(validationCommon.NamePrefixedGVK)
	}

	return admission.Allowed("")
}

// InjectDecoder implements admission.DecoderInjector interface.
// A decoder will be automatically injected.
func (v *ValidatingAdmission) InjectDecoder(d *admission.Decoder) error {
	v.decoder = d
	return nil
}

// Check if our ValidatingAdmission implements necessary interface
var _ admission.Handler = &ValidatingAdmission{}
var _ admission.DecoderInjector = &ValidatingAdmission{}
