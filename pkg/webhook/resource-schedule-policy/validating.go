package resource_schedule_policy

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	validationCommon "harmonycloud.cn/stellaris/pkg/common/validation"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var multiSchedulePolicyLog = logf.Log.WithName("webhook_mSchedulePolicy")

// ValidatingAdmission validates multiClusterResource object when creating/updating/deleting.
type ValidatingAdmission struct {
	decoder *admission.Decoder
}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	policy := &v1alpha1.MultiClusterResourceSchedulePolicy{}
	err := v.decoder.Decode(req, policy)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	multiSchedulePolicyLog.Info("Validating SchedulePolicy:", policy.Name, ", event:", req.Operation)

	if errs := validationCommon.ValidateMultiSchedulePolicy(policy); len(errs) > 0 {
		errMsg := fmt.Sprintf("invalid SchedulePolicy name(%s): %s", policy.Name, strings.Join(errs, ";"))
		multiSchedulePolicyLog.Info(errMsg)
		return admission.Denied(errMsg)
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
