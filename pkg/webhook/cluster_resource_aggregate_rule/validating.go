package cluster_resource_aggregate_rule

import (
	"context"
	"net/http"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	validationCommon "harmonycloud.cn/stellaris/pkg/common/validation"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ValidatingAdmission validates clusterResource object when creating/updating/deleting.
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
	klog.V(2).Infof("Validating clusterResource(%s) for request: %s", aggregateRule.Name, req.Operation)
	// TODO(chenkun) currently we only validate whether it contains CUE.
	if len(aggregateRule.Spec.Rule.Cue) <= 0 {
		klog.Error(validationCommon.CueIsEmpty)
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
