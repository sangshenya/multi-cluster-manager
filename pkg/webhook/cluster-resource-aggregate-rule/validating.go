package cluster_resource_aggregate_rule

import (
	"context"
	"errors"
	"net/http"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	validationCommon "harmonycloud.cn/stellaris/pkg/common/validation"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var webhookMultiRuleLog = logf.Log.WithName("webhook_mRule")

// ValidatingAdmission validates aggregateRule object when creating/updating/deleting.
type ValidatingAdmission struct {
	decoder *admission.Decoder
}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	aggregateRule := &v1alpha1.MultiClusterResourceAggregateRule{}
	err := v.decoder.Decode(req, aggregateRule)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	webhookMultiRuleLog.Info("Validating aggregateRule:", aggregateRule.Name, ",for request:", req.Operation)
	// TODO(chenkun) currently we only validate whether it contains CUE.
	if len(aggregateRule.Spec.Rule.Cue) == 0 {
		webhookMultiRuleLog.Error(errors.New(validationCommon.CueIsEmpty), validationCommon.CueIsEmpty)
		return admission.Denied(validationCommon.CueIsEmpty)
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
