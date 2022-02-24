package config

import (
	"errors"
	"sync"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var informerResourceConfig *InformerResourceConfig

func init() {
	informerResourceConfig = &InformerResourceConfig{ConfigMap: make(map[string]*v1alpha1.ResourceAggregatePolicySpec)}
}

type InformerResourceConfig struct {
	ConfigMap map[string]*v1alpha1.ResourceAggregatePolicySpec
	lock      sync.RWMutex
}

func (c *InformerResourceConfig) AddConfig(policy *v1alpha1.ResourceAggregatePolicy) error {
	if policy.Spec.ResourceRef == nil {
		return errors.New("gvk is empty")
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	gvkString := managerCommon.GvkLabelString(policy.Spec.ResourceRef)
	c.ConfigMap[gvkString] = &policy.Spec
	return nil
}

func (c *InformerResourceConfig) RemoveConfig(policy *v1alpha1.ResourceAggregatePolicy) error {
	if policy.Spec.ResourceRef == nil {
		return errors.New("gvk is empty")
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	gvkString := managerCommon.GvkLabelString(policy.Spec.ResourceRef)
	delete(c.ConfigMap, gvkString)
	return nil
}

func (c *InformerResourceConfig) GetConfig(resourceRef *metav1.GroupVersionKind) *v1alpha1.ResourceAggregatePolicySpec {
	c.lock.RLock()
	defer c.lock.RUnlock()
	gvkString := managerCommon.GvkLabelString(resourceRef)
	rc, ok := c.ConfigMap[gvkString]
	if !ok {
		return nil
	}
	return rc
}
