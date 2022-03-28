package cluster

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	validationCommon "harmonycloud.cn/stellaris/pkg/common/validation"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var webhookClusterLog = logf.Log.WithName("webhook_cluster")

// ValidatingAdmission validates clusterResource object when creating/updating/deleting.
type ValidatingAdmission struct {
	decoder *admission.Decoder
}

// Handle implements admission.Handler interface.
// It yields a response to an AdmissionRequest.
func (v *ValidatingAdmission) Handle(ctx context.Context, req admission.Request) admission.Response {
	cluster := &v1alpha1.Cluster{}
	err := v.decoder.Decode(req, cluster)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	webhookClusterLog.Info("Validating cluster:", cluster.Name, ", for request:", req.Operation)
	// validate clusterResource name
	if errs := validationCommon.ValidateClusterProxyURL(cluster.Spec.ApiServer); len(errs) > 0 {
		errMsg := fmt.Sprintf("invalid cluster name(%s): %s", cluster.Name, strings.Join(errs, ";"))
		webhookClusterLog.Info(errMsg)
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
