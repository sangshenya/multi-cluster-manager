package multi_cluster_resource_binding

import (
	"context"
	"errors"
	"net/http"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	validationCommon "harmonycloud.cn/stellaris/pkg/common/validation"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var webhookBindingLog = logf.Log.WithName("webhook_binding")

// ValidatingAdmission validates multiClusterResource object when creating/updating/deleting.
type ValidatingAdmission struct {
	decoder *admission.Decoder
}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	binding := &v1alpha1.MultiClusterResourceBinding{}
	err := v.decoder.Decode(req, binding)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	webhookBindingLog.Info("Validating MultiClusterResourceBinding:", binding.Name, ", for request: ", req.Operation)

	// empty binding can not be create
	if len(binding.Spec.Resources) == 0 {
		webhookBindingLog.Error(errors.New(validationCommon.ResourceIsNil), validationCommon.ResourceIsNil)
		return admission.Denied(validationCommon.ResourceIsNil)
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
