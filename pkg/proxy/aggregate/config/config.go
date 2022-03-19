package config

import (
	"errors"
	"sync"

	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	managerCommon "harmonycloud.cn/stellaris/pkg/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

/*
 * ResourceConfig
 * {
 * 		"policy-gvk": {
 *			"policyNamespace.policyName": spec,
 *			...
 *		},
 *		...
 * }
 */
var ResourceConfig = InformerResourceConfig{
	ConfigMap: make(map[string]*Resource),
}

type InformerResourceConfig struct {
	ConfigMap map[string]*Resource
	lock      sync.RWMutex
}

type Resource struct {
	ResourceMap map[string]*v1alpha1.ResourceAggregatePolicySpec
}

func (c *InformerResourceConfig) AddOrUpdateConfig(policy *v1alpha1.ResourceAggregatePolicy) error {
	if policy.Spec.ResourceRef == nil {
		return errors.New("gvk is empty")
	}
	gvkString := managerCommon.GvkLabelString(policy.Spec.ResourceRef)
	c.lock.Lock()
	defer c.lock.Unlock()
	resourceMap, ok := c.ConfigMap[gvkString]
	if !ok {
		resourceMap = &Resource{
			ResourceMap: make(map[string]*v1alpha1.ResourceAggregatePolicySpec),
		}
	}
	resourceMap.ResourceMap[resourceKey(policy.Namespace, policy.Name)] = &policy.Spec

	c.ConfigMap[gvkString] = resourceMap
	return nil
}

func resourceKey(ns, name string) string {
	return ns + "." + name
}

func (c *InformerResourceConfig) RemoveConfig(policy *v1alpha1.ResourceAggregatePolicy) error {
	if policy.Spec.ResourceRef == nil {
		return errors.New("gvk is empty")
	}
	gvkString := managerCommon.GvkLabelString(policy.Spec.ResourceRef)
	c.lock.Lock()
	defer c.lock.Unlock()

	resourceMap, ok := c.ConfigMap[gvkString]
	if !ok {
		return nil
	}
	key := resourceKey(policy.Namespace, policy.Name)
	_, ok = resourceMap.ResourceMap[key]
	if !ok {
		return nil
	}

	if len(resourceMap.ResourceMap) == 1 {
		delete(c.ConfigMap, gvkString)
		return nil
	}
	delete(resourceMap.ResourceMap, key)
	c.ConfigMap[gvkString] = resourceMap
	return nil
}

func (c *InformerResourceConfig) GetConfig(resourceRef *metav1.GroupVersionKind) []*v1alpha1.ResourceAggregatePolicySpec {
	c.lock.RLock()
	defer c.lock.RUnlock()
	gvkString := managerCommon.GvkLabelString(resourceRef)
	resourceMap, ok := c.ConfigMap[gvkString]
	if !ok {
		return nil
	}
	specList := make([]*v1alpha1.ResourceAggregatePolicySpec, 0)
	for _, v := range resourceMap.ResourceMap {
		specList = append(specList, v)
	}
	return specList
}
