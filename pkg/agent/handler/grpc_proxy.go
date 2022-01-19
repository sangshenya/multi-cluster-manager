package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"harmonycloud.cn/stellaris/config"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"harmonycloud.cn/stellaris/pkg/agent/addons"
	agentconfig "harmonycloud.cn/stellaris/pkg/agent/config"
	agentStream "harmonycloud.cn/stellaris/pkg/agent/stream"
	multclusterclient "harmonycloud.cn/stellaris/pkg/client/clientset/versioned"
	"harmonycloud.cn/stellaris/pkg/model"
	"harmonycloud.cn/stellaris/pkg/util/agent"
	"harmonycloud.cn/stellaris/pkg/util/common"
)

var registerLog = logf.Log.WithName("agent_register")

func Register() error {
	registerLog.Info(fmt.Sprintf("start register cluster(%s)", agentconfig.AgentConfig.Cfg.ClusterName))
	stream := agentStream.GetConnection()
	if stream == nil {
		err := errors.New("get stream failed")
		registerLog.Error(err, "register")
		return err
	}
	addonInfo := &model.RegisterRequest{}
	if agentconfig.AgentConfig.Cfg.AddonPath != "" {
		addonConfig, err := agent.GetAddonConfig(agentconfig.AgentConfig.Cfg.AddonPath)
		if err != nil {
			registerLog.Error(err, "get addons config failed")
			return err
		}
		addonsList := addons.LoadAddon(addonConfig)
		addonInfo.Addons = addonsList
	}

	request, err := common.GenerateRequest("Register", addonInfo, agentconfig.AgentConfig.Cfg.ClusterName)
	if err != nil {
		registerLog.Error(err, "create request failed")
		return err
	}
	if err := stream.Send(request); err != nil {
		registerLog.Error(err, "send request failed")
		return err
	}

	return nil
}

func RecvRegisterResponse(response *config.Response) {
	registerLog.Info(fmt.Sprintf("get response from server:%s", response.String()))

	var err error
	if response.Type != model.RegisterSuccess.String() {
		err = errors.New(response.Body)
		registerLog.Error(err, "response is not register success")
		return
	}

	registerLog.Info(fmt.Sprintf("start send heartbeat"))
	go HeartbeatStart()

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
	if err := syncClusterResources(ctx, agentClient, resourceList.ClusterResources); err != nil {
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

func syncClusterResources(ctx context.Context, agentClient *multclusterclient.Clientset, clusterResourceList []string) error {
	for _, str := range clusterResourceList {
		clusterResource := &v1alpha1.ClusterResource{}
		err := json.Unmarshal([]byte(str), clusterResource)
		if err != nil {
			registerLog.Error(err, "get cluster resource failed")
			return err
		}
		_, err = agentClient.MulticlusterV1alpha1().ClusterResources(clusterResource.GetNamespace()).Create(ctx, clusterResource, metav1.CreateOptions{})
		if err != nil {
			if !apierrors.IsAlreadyExists(err) {
				return err
			}
			_, err = agentClient.MulticlusterV1alpha1().ClusterResources(clusterResource.GetNamespace()).Update(ctx, clusterResource, metav1.UpdateOptions{})
			if err != nil {
				return err
			}
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
