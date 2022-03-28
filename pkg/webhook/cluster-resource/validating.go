package cluster_resource

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	validationCommon "harmonycloud.cn/stellaris/pkg/common/validation"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var webhookClusterResourceLog = logf.Log.WithName("webhook_ClusterResource")

// ValidatingAdmission validates clusterResource object when creating/updating/deleting.
type ValidatingAdmission struct {
	decoder *admission.Decoder
}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	clusterResource := &v1alpha1.ClusterResource{}
	err := v.decoder.Decode(req, clusterResource)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	webhookClusterResourceLog.Info("Validating clusterResource:", clusterResource.Name, ", for request: %s", req.Operation)
	// validate clusterResource name
	if errs := validationCommon.ValidateClusterResourceName(clusterResource.Name); len(errs) > 0 {
		errMsg := fmt.Sprintf("invalid clusterResource name(%s): %s", clusterResource.Name, strings.Join(errs, ";"))
		webhookClusterResourceLog.Error(errors.New(errMsg), errMsg)
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
