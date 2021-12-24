package common

import (
	"context"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"

	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	sliceutil "harmonycloud.cn/stellaris/pkg/util/slice"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func ShouldAddFinalizer(object client.Object) bool {
	if !sliceutil.ContainsString(object.GetFinalizers(), managerCommon.FinalizerName) && object.GetDeletionTimestamp().IsZero() {
		return true
	}
	return false
}

// AddFinalizers add multi finalizer, do nothing when the multi finalizer is already exists
func AddFinalizer(ctx context.Context, clientSet client.Client, object client.Object) error {
	if sliceutil.ContainsString(object.GetFinalizers(), managerCommon.FinalizerName) {
		return nil
	}
	object.SetFinalizers(append(object.GetFinalizers(), managerCommon.FinalizerName))
	return clientSet.Update(ctx, object)
}

// RemoveFinalizer remove multi finalizer, do nothing when the multi finalizer isn`t already exists
func RemoveFinalizer(ctx context.Context, clientSet client.Client, object client.Object) error {
	if !sliceutil.ContainsString(object.GetFinalizers(), managerCommon.FinalizerName) {
		return nil
	}
	object.SetFinalizers(sliceutil.RemoveString(object.GetFinalizers(), managerCommon.FinalizerName))
	return clientSet.Update(ctx, object)
}

func ReQueueResult(err error) (ctrl.Result, error) {
	return ctrl.Result{
		Requeue:      true,
		RequeueAfter: 30 * time.Second,
	}, err
}
