package multi_cluster_resource

import (
	"context"
	"errors"
	"net/http"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"

	"harmonycloud.cn/stellaris/pkg/common/helper"

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

	if req.Operation == admissionv1.Update {
		oldMultiClusterResource := &v1alpha1.MultiClusterResource{}
		err = v.decoder.DecodeRaw(req.OldObject, oldMultiClusterResource)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		err = updateValidate(oldMultiClusterResource, multiClusterResource)
		if err != nil {
			klog.Error(err)
			return admission.Denied(err.Error())
		}
	}

	return admission.Allowed("")
}

func updateValidate(oldMultiClusterResource, multiClusterResource *v1alpha1.MultiClusterResource) error {
	if oldMultiClusterResource.Spec.ResourceRef.String() != multiClusterResource.Spec.ResourceRef.String() {
		return errors.New(validationCommon.CanNotChangedGVK)
	}

	oldResource, err := helper.GetResourceForRawExtension(oldMultiClusterResource.Spec.Resource)
	if err != nil {
		return errors.New(validationCommon.ResourceMarshalFail + ", error:" + err.Error())
	}

	newResource, err := helper.GetResourceForRawExtension(multiClusterResource.Spec.Resource)
	if err != nil {
		return errors.New(validationCommon.ResourceMarshalFail + ", error:" + err.Error())
	}

	if oldResource.GetName() != newResource.GetName() || oldResource.GetNamespace() != newResource.GetNamespace() {
		return errors.New(validationCommon.CanNotChangedIdentity)
	}
	return nil
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
