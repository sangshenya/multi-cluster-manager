package multi_cluster_resource

import (
	"context"
	"errors"
	"net/http"
	"strings"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	admissionv1 "k8s.io/api/admission/v1"

	"harmonycloud.cn/stellaris/pkg/common/helper"

	managerCommon "harmonycloud.cn/stellaris/pkg/common"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	validationCommon "harmonycloud.cn/stellaris/pkg/common/validation"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var webhookClusterResourceLog = logf.Log.WithName("webhook_mClusterResource")

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
	webhookClusterResourceLog.Info("Validating multiClusterResource:", multiClusterResource.Name, ", for request:", req.Operation)

	if multiClusterResource.Spec.Resource == nil {
		webhookClusterResourceLog.Error(errors.New(validationCommon.ResourceIsNil), validationCommon.ResourceIsNil)
		return admission.Denied(validationCommon.ResourceIsNil)
	}

	if multiClusterResource.Spec.ResourceRef == nil {
		webhookClusterResourceLog.Error(errors.New(validationCommon.ResourceRefIsNil), validationCommon.ResourceRefIsNil)
		return admission.Denied(validationCommon.ResourceRefIsNil)
	}

	if !strings.HasPrefix(multiClusterResource.GetName(), managerCommon.GvkLabelString(multiClusterResource.Spec.ResourceRef)) {
		webhookClusterResourceLog.Error(errors.New(validationCommon.NamePrefixedGVK), validationCommon.NamePrefixedGVK)
		return admission.Denied(validationCommon.NamePrefixedGVK)
	}

	if req.Operation == admissionv1.Update {
		oldMultiClusterResource := &v1alpha1.MultiClusterResource{}
		err = v.decoder.DecodeRaw(req.OldObject, oldMultiClusterResource)
		if err != nil {
			webhookClusterResourceLog.Error(err, "mClusterResource update")
			return admission.Errored(http.StatusBadRequest, err)
		}
		err = updateValidate(oldMultiClusterResource, multiClusterResource)
		if err != nil {
			webhookClusterResourceLog.Error(err, "mClusterResource update")
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
