package monitor

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"harmonycloud.cn/stellaris/pkg/common/helper"
)

func ConfigHeadlessService(ctx context.Context) error {

	client, err := helper.GetKubeClient()
	if err != nil {
		return err
	}

	namespace, err := helper.GetOperatorNamespace()
	if err != nil {
		return err
	}

	name, err := helper.GetOperatorName()
	if err != nil {
		return err
	}

	nameKey := types.NamespacedName{
		Namespace: namespace,
		Name:      name,
	}

	// get pod info
	pod := &corev1.Pod{}
	if err = client.Get(ctx, nameKey, pod); err != nil {
		return err
	}

	// get endpoints and update subset
	endpoints := &corev1.Endpoints{}
	if err = client.Get(ctx, nameKey, endpoints); err != nil {
		return err
	}

	endpointSubset := corev1.EndpointSubset{
		Addresses: []corev1.EndpointAddress{
			corev1.EndpointAddress{
				IP:       pod.Status.PodIP,
				Hostname: pod.Spec.Hostname,
				NodeName: &pod.Spec.NodeName,
			},
		},
		NotReadyAddresses: nil,
		Ports: []corev1.EndpointPort{
			corev1.EndpointPort{
				Name:     "grpc",
				Protocol: corev1.ProtocolTCP,
				Port:     8080,
			},
		},
	}

	endpoints.Subsets = []corev1.EndpointSubset{
		endpointSubset,
	}

	err = client.Update(ctx, endpoints)
	if err != nil {
		return err
	}

	return nil
}
