package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"harmonycloud.cn/stellaris/pkg/agent/send"

	clusterResourceController "harmonycloud.cn/stellaris/pkg/controller/cluster-resource"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"harmonycloud.cn/stellaris/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	agentconfig "harmonycloud.cn/stellaris/pkg/agent/config"
	multclusterclient "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	"harmonycloud.cn/stellaris/pkg/model"
)

var registerLog = logf.Log.WithName("agent_register")

func RecvRegisterResponse(response *config.Response) {
	registerLog.Info(fmt.Sprintf("get response from server:%s", response.String()))

	var err error
	if response.Type != model.RegisterSuccess.String() {
		err = errors.New(response.Body)
		registerLog.Error(err, "response is not register success")
		return
	}

	registerLog.Info(fmt.Sprintf("start send heartbeat"))
	go send.HeartbeatStart()

	err = dealResponse(agentconfig.AgentConfig.AgentClient, response)
	if err != nil {
		registerLog.Error(err, "deal response failed")
	}
}

func dealResponse(agentClient *multclusterclient.Clientset, response *config.Response) error {
	if len(response.Body) <= 0 {
		return nil
	}
	resources := &model.RegisterResponse{}
	err := json.Unmarshal([]byte(response.Body), resources)
	if err != nil {
		return err
	}
	return syncResource(agentClient, resources)
}

func syncResource(agentClient *multclusterclient.Clientset, resourceList *model.RegisterResponse) error {
	ctx := context.Background()
	if err := syncClusterResourcesList(ctx, agentClient, resourceList.ClusterResources); err != nil {
		return err
	}

	if err := syncPolicys(ctx, agentClient, resourceList.MultiClusterResourceAggregatePolicies); err != nil {
		return err
	}

	if err := syncRules(ctx, agentClient, resourceList.MultiClusterResourceAggregateRules); err != nil {
		return err
	}
	return nil
}

func syncClusterResourcesList(ctx context.Context, agentClient *multclusterclient.Clientset, clusterResourceList []string) error {
	for _, str := range clusterResourceList {
		registerLog.Info(fmt.Sprintf("start sync register response clusterResource"))
		clusterResource := &v1alpha1.ClusterResource{}
		err := json.Unmarshal([]byte(str), clusterResource)
		if err != nil {
			registerLog.Error(err, "get cluster resource failed")
			return err
		}
		resource, err := clusterResourceController.GetClusterResourceObjectForRawExtension(clusterResource)
		if err != nil {
			continue
		}
		clusterResource.SetNamespace(resource.GetNamespace())
		err = clusterResourceController.SyncAgentClusterResource(ctx, agentClient, clusterResource)
		if err != nil {
			registerLog.Error(err, fmt.Sprintf("sync ClusterResource(%s:%s) failed", clusterResource.Namespace, clusterResource.Name))
			return err
		} else {
			registerLog.Info(fmt.Sprintf("sync ClusterResource(%s:%s) success", clusterResource.Namespace, clusterResource.Name))
		}
	}
	return nil
}

func syncPolicys(ctx context.Context, agentClient *multclusterclient.Clientset, policyList []string) error {
	for _, str := range policyList {
		policy := &v1alpha1.MultiClusterResourceAggregatePolicy{}
		err := json.Unmarshal([]byte(str), policy)
		if err != nil {
			registerLog.Error(err, "get policy failed")
			return err
		}
		_, err = agentClient.MulticlusterV1alpha1().MultiClusterResourceAggregatePolicies(policy.GetNamespace()).Create(ctx, policy, metav1.CreateOptions{})
		if err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return err
			}
			_, err = agentClient.MulticlusterV1alpha1().MultiClusterResourceAggregatePolicies(policy.GetNamespace()).Update(ctx, policy, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func syncRules(ctx context.Context, agentClient *multclusterclient.Clientset, ruleList []string) error {
	for _, str := range ruleList {
		rule := &v1alpha1.MultiClusterResourceAggregateRule{}
		err := json.Unmarshal([]byte(str), rule)
		if err != nil {
			registerLog.Error(err, "get rule failed")
			return err
		}
		_, err = agentClient.MulticlusterV1alpha1().MultiClusterResourceAggregateRules(rule.GetNamespace()).Create(ctx, rule, metav1.CreateOptions{})
		if err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return err
			}
			_, err = agentClient.MulticlusterV1alpha1().MultiClusterResourceAggregateRules(rule.GetNamespace()).Update(ctx, rule, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
		}
	}
	return nil
}
