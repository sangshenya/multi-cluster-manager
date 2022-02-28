package resource_schedule_policy

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	apicommon "harmonycloud.cn/stellaris/pkg/apis/multicluster/common"
	"harmonycloud.cn/stellaris/pkg/apis/multicluster/v1alpha1"
	pkgcommon "harmonycloud.cn/stellaris/pkg/common"
	controllerCommon "harmonycloud.cn/stellaris/pkg/controller/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

type Reconciler struct {
	client.Client
	log    logr.Logger
	Scheme *runtime.Scheme
}

func (r *Reconciler) Reconcile(ctx context.Context, request ctrl.Request) (ctrl.Result, error) {
	r.log.V(4).Info(fmt.Sprintf("Start Reconciling MultiClusterResourceSchedulePolicy(%s:%s)", request.Namespace, request.Name))
	defer r.log.V(4).Info(fmt.Sprintf("End Reconciling MultiClusterResourceSchedulePolicy(%s:%s)", request.Namespace, request.Name))

	schedulePolicy := &v1alpha1.MultiClusterResourceSchedulePolicy{}
	err := r.Client.Get(ctx, request.NamespacedName, schedulePolicy)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if err = r.updateLastModifyTime(ctx, schedulePolicy); err != nil {
		r.log.Error(err, fmt.Sprintf("fail to update schedulePolicy(%s:%s) status", schedulePolicy.Namespace, schedulePolicy.Name))
		return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
	}
	if schedulePolicy.Spec.Reschedule {
		if err = r.doSchedule(ctx, schedulePolicy); err != nil {
			r.log.Error(err, fmt.Sprintf("fail to reschedule schedulePolicy(%s:%s)", schedulePolicy.Namespace, schedulePolicy.Name))
			return ctrl.Result{Requeue: true, RequeueAfter: 5 * time.Second}, err
		}
	}
	return ctrl.Result{}, nil
}

// schedule by assign or clusterset,
func (r *Reconciler) doSchedule(ctx context.Context, policy *v1alpha1.MultiClusterResourceSchedulePolicy) error {
	// TODO label
	var (
		binding *v1alpha1.MultiClusterResourceBinding
		err     error
	)
	switch policy.Spec.ClusterSource {
	case v1alpha1.ClusterSourceTypeAssign:
		binding, err = r.scheduleByAssign(ctx, policy)
		if err != nil {
			err = fmt.Errorf(fmt.Sprintf("fail to do schedule(%s:%s)", policy.Namespace, policy.Name), err)
			return err
		}
	case v1alpha1.ClusterSourceTypeClusterset:
		binding, err = r.scheduleByClusterSet(ctx, policy)
		if err != nil {
			err = fmt.Errorf(fmt.Sprintf("fail to do schedule(%s:%s)", policy.Namespace, policy.Name), err)
			return err
		}
	}
	// create or update only when changed
	same := r.compareBinding(ctx, binding)
	if !same {
		err = r.createOrUpdateBinding(ctx, binding)
		if err != nil {
			err = fmt.Errorf(fmt.Sprintf("create or update binding failed, schedule(%s:%s)", policy.Namespace, policy.Name), err)
			return err
		}
	}

	err = r.updateLastScheduleTime(ctx, policy)
	if err != nil {
		err = fmt.Errorf(fmt.Sprintf("update last scheduleTime failed, schedule(%s:%s)", policy.Namespace, policy.Name), err)
		return err
	}

	return nil
}

// schedule by duplicated or weighted
func (r *Reconciler) scheduleByAssign(ctx context.Context, policy *v1alpha1.MultiClusterResourceSchedulePolicy) (*v1alpha1.MultiClusterResourceBinding, error) {
	switch policy.Spec.ScheduleMode {
	case v1alpha1.ScheduleModeTypeDuplicated:
		binding, err := r.doTypeDuplicated(ctx, policy)
		if err != nil {
			return nil, err
		}
		return binding, nil
	case v1alpha1.ScheduleModeTypeWeighted:
		binding, err := r.doTypeWeighted(ctx, policy)
		if err != nil {
			return nil, err
		}
		return binding, nil
	default:
		return nil, nil
	}
}

// schedule by clusterrole or clusterselector
func (r *Reconciler) scheduleByClusterSet(ctx context.Context, policy *v1alpha1.MultiClusterResourceSchedulePolicy) (*v1alpha1.MultiClusterResourceBinding, error) {
	clusterSet := &v1alpha1.ClusterSet{}
	clusterSetNamespacedName := types.NamespacedName{
		Name: policy.Spec.Clusterset,
	}
	if err := r.Client.Get(ctx, clusterSetNamespacedName, clusterSet); err != nil {
		return nil, err
	}

	if len(clusterSet.Spec.Clusters) > 0 {
		binding, err := r.doTypeClusterRole(ctx, policy, clusterSet)
		if err != nil {
			return nil, err
		}
		return binding, nil
	} else if len(clusterSet.Spec.Selector.Labels) > 0 {
		binding, err := r.doTypeClusterSelector(ctx, policy, clusterSet)
		if err != nil {
			return nil, err
		}
		return binding, nil
	}

	return nil, nil
}

func (r *Reconciler) doTypeDuplicated(ctx context.Context, policy *v1alpha1.MultiClusterResourceSchedulePolicy) (*v1alpha1.MultiClusterResourceBinding, error) {
	failClusterIndex, unavailableFailoverClusters, err := r.checkClusters(ctx, policy)
	if err != nil {
		r.log.Error(err, "check clusters err")
		return nil, err
	}
	binding, err := r.generateBindingByDuplicated(ctx, policy, failClusterIndex, unavailableFailoverClusters)
	if err != nil {
		r.log.Error(err, "generate resource binding err")
		return nil, err
	}
	return binding, nil

}

func (r *Reconciler) doTypeWeighted(ctx context.Context, policy *v1alpha1.MultiClusterResourceSchedulePolicy) (*v1alpha1.MultiClusterResourceBinding, error) {
	failClusterIndex, unavailableFailoverClusters, err := r.checkClusters(ctx, policy)
	if err != nil {
		r.log.Error(err, "check clusters err")
		return nil, err
	}
	binding, err := r.generateBindingByWeighted(ctx, policy, failClusterIndex, unavailableFailoverClusters)
	if err != nil {
		r.log.Error(err, "generate resource binding err")
		return nil, err
	}

	return binding, nil
}

func (r *Reconciler) doTypeClusterRole(ctx context.Context, policy *v1alpha1.MultiClusterResourceSchedulePolicy, clusterSet *v1alpha1.ClusterSet) (*v1alpha1.MultiClusterResourceBinding, error) {
	failClusterIndex, unavailableFailoverClusters, err := r.checkClusters(ctx, policy)
	if err != nil {
		r.log.Error(err, "check clusters err")
		return nil, err
	}
	binding, err := r.generateBindingByClusterRole(ctx, policy, clusterSet, failClusterIndex, unavailableFailoverClusters)
	if err != nil {
		r.log.Error(err, "generate resource binding err")
		return nil, err
	}

	return binding, nil
}

func (r *Reconciler) doTypeClusterSelector(ctx context.Context, policy *v1alpha1.MultiClusterResourceSchedulePolicy, clusterSet *v1alpha1.ClusterSet) (*v1alpha1.MultiClusterResourceBinding, error) {
	failClusterIndex, unavailableFailoverClusters, err := r.checkClusters(ctx, policy)
	if err != nil {
		r.log.Error(err, "check clusters err")
		return nil, err
	}
	binding, err := r.generateBindingByClusterSelector(ctx, policy, clusterSet, failClusterIndex, unavailableFailoverClusters)
	if err != nil {
		r.log.Error(err, "generate resource binding err")
		return nil, err
	}

	return binding, nil
}

// calculate index of unavailableCluster in policy,unavailable clusters in failover policy
func (r *Reconciler) checkClusters(ctx context.Context, policy *v1alpha1.MultiClusterResourceSchedulePolicy) ([]int, []string, error) {
	// check clusters
	var (
		unavailableClusters         []string
		unavailableIndex            []int
		unavailableFailoverClusters []string
	)

	if policy.Spec.ClusterSource == v1alpha1.ClusterSourceTypeClusterset {
		clusterSet := &v1alpha1.ClusterSet{}
		clusterSetNamespacedName := types.NamespacedName{
			Name: policy.Spec.Clusterset,
		}
		if err := r.Client.Get(ctx, clusterSetNamespacedName, clusterSet); err != nil {
			return nil, nil, err
		}
		clusterList := r.getClusterListByClusterSet(ctx, clusterSet)
		for i, instance := range clusterList {
			err := r.checkCluster(ctx, instance)
			if err != nil {
				unavailableClusters = append(unavailableClusters, instance)
				unavailableIndex = append(unavailableIndex, i)
			}
		}
	} else {
		for i, instance := range policy.Spec.Policy {
			err := r.checkCluster(ctx, instance.Name)
			if err != nil {
				unavailableClusters = append(unavailableClusters, instance.Name)
				unavailableIndex = append(unavailableIndex, i)
			}
		}
	}
	if len(policy.Spec.FailoverPolicy) > 0 && len(unavailableClusters) > 0 {
		failoverCount, unavailableFailoverClusters := r.failoverPolicyCheck(ctx, policy)
		if len(unavailableClusters) > failoverCount {
			return nil, nil, fmt.Errorf("clusters unavailable: %s,but %d failover clusters available", fmt.Sprint(unavailableClusters), failoverCount)
		}
		return unavailableIndex, unavailableFailoverClusters, nil

	} else if len(unavailableClusters) > 0 {
		return nil, nil, fmt.Errorf("clusters unavailable: %s", fmt.Sprint(unavailableClusters))
	}

	return unavailableIndex, unavailableFailoverClusters, nil
}

// check single cluster
func (r *Reconciler) checkCluster(ctx context.Context, clusterName string) error {
	cluster := &v1alpha1.Cluster{}
	clusterNamespacedName := types.NamespacedName{
		Name: clusterName,
	}
	err := r.Client.Get(ctx, clusterNamespacedName, cluster)
	if err != nil {
		return err
	}
	if cluster.Status.Status != v1alpha1.OnlineStatus {
		return fmt.Errorf("cluster %v offline", clusterName)
	}
	return nil
}

func (r *Reconciler) generateBindingByDuplicated(ctx context.Context, policy *v1alpha1.MultiClusterResourceSchedulePolicy, failIndex []int, unavailableFailoverClusters []string) (*v1alpha1.MultiClusterResourceBinding, error) {
	binding := r.addBindingMeta(policy)

	for i, resourceInstance := range policy.Spec.Resources {
		binding.Spec.Resources = append(binding.Spec.Resources, v1alpha1.MultiClusterResourceBindingResource{Name: resourceInstance.Name})
		indexModel := controllerCommon.FailoverPolicyIndex{}
		// get clusters for resource
		for j, policyInstance := range policy.Spec.Policy {
			// there are unavailable clusters in policy
			if failIndex != nil {
				//	cluster needs failover
				if indexModel.DoneIndex < len(failIndex) && j == failIndex[indexModel.DoneIndex] {
					err := r.doFailoverPolicy(ctx, &indexModel, policy.Spec.FailoverPolicy, &binding.Spec.Resources[i], unavailableFailoverClusters)
					if err != nil {
						return nil, err
					}
				} else {
					binding.Spec.Resources[i].Clusters = append(binding.Spec.Resources[i].Clusters, v1alpha1.MultiClusterResourceBindingCluster{Name: policyInstance.Name})
				}
			} else {
				binding.Spec.Resources[i].Clusters = append(binding.Spec.Resources[i].Clusters, v1alpha1.MultiClusterResourceBindingCluster{Name: policyInstance.Name})
			}

			// replace replicas
			err := r.replaceResourceReplicasField(ctx, policy, resourceInstance.Name, &binding.Spec.Resources[i].Clusters[j])
			if err != nil {
				return nil, err
			}

			// do namespace mapping
			err = r.checkNamespaceMapping(ctx, policy, policyInstance.Name, &binding.Spec.Resources[i].Clusters[j])
			if err != nil {
				return nil, err
			}
		}
	}
	return binding, nil
}

func (r *Reconciler) generateBindingByWeighted(ctx context.Context, policy *v1alpha1.MultiClusterResourceSchedulePolicy, failIndex []int, unavailableFailoverClusters []string) (*v1alpha1.MultiClusterResourceBinding, error) {
	binding := r.addBindingMeta(policy)
	totalWeight, diff := r.calculateWeight(policy)

	for i, resourceInstance := range policy.Spec.Resources {
		diffReplicas := diff
		binding.Spec.Resources = append(binding.Spec.Resources, v1alpha1.MultiClusterResourceBindingResource{Name: resourceInstance.Name})
		indexModel := controllerCommon.FailoverPolicyIndex{}
		sortPolicy := r.doSortPolicyList(policy)
		tempModel := &controllerCommon.FirstReplaceReplicasModel{
			ResourceName: resourceInstance.Name,
			TotalWeight:  totalWeight,
			DiffReplicas: diffReplicas,
		}
		for j, policyInstance := range policy.Spec.Policy {
			if failIndex != nil {
				if indexModel.DoneIndex < len(failIndex) && j == failIndex[indexModel.DoneIndex] {
					err := r.doFailoverPolicy(ctx, &indexModel, policy.Spec.FailoverPolicy, &binding.Spec.Resources[i], unavailableFailoverClusters)
					if err != nil {
						return nil, err
					}
				} else {
					binding.Spec.Resources[i].Clusters = append(binding.Spec.Resources[i].Clusters, v1alpha1.MultiClusterResourceBindingCluster{Name: policyInstance.Name})
				}
			} else {
				binding.Spec.Resources[i].Clusters = append(binding.Spec.Resources[i].Clusters, v1alpha1.MultiClusterResourceBindingCluster{Name: policyInstance.Name})
			}
			// step1:replace replicas directly by weight
			err := r.firstReplaceReplicasByWeight(ctx, policy, &binding.Spec.Resources[i].Clusters[j], &policyInstance, tempModel)
			if err != nil {
				return nil, err
			}
			err = r.checkNamespaceMapping(ctx, policy, policyInstance.Name, &binding.Spec.Resources[i].Clusters[j])
			if err != nil {
				return nil, err
			}
		}
		if tempModel.DiffReplicas != 0 {
			// step2:assign replicas if total-replicas is not equal to policy.spec.replicas
			err := r.replaceReplicasByWeight(policy, &binding.Spec.Resources[i], tempModel.DiffReplicas, sortPolicy)
			if err != nil {
				return nil, err
			}
		}
	}
	return binding, nil
}

// calculate replicas,and replace
func (r *Reconciler) firstReplaceReplicasByWeight(ctx context.Context, policy *v1alpha1.MultiClusterResourceSchedulePolicy, bindingResourceCluster *v1alpha1.MultiClusterResourceBindingCluster, policyInstance *v1alpha1.SchedulePolicy, model *controllerCommon.FirstReplaceReplicasModel) error {
	resource := &v1alpha1.MultiClusterResource{}
	resourceNamespacedName := types.NamespacedName{
		Name:      model.ResourceName,
		Namespace: policy.Namespace,
	}
	err := r.Client.Get(ctx, resourceNamespacedName, resource)
	if err != nil {
		return err
	}
	// replace replicas
	if len(resource.Spec.ReplicasField) > 0 {
		replicas := int((float64(policy.Spec.Replicas) / float64(model.TotalWeight)) * float64(policyInstance.Weight))
		if replicas < policyInstance.Min {
			model.DiffReplicas -= policyInstance.Min - replicas
			replicas = policyInstance.Min

		} else if replicas > policyInstance.Max {
			model.DiffReplicas += replicas - policyInstance.Max
			replicas = policyInstance.Max

		}

		bindingResourceCluster.Override = append(bindingResourceCluster.Override, apicommon.JSONPatch{
			Path:  resource.Spec.ReplicasField,
			Op:    pkgcommon.BindingOpReplace,
			Value: strconv.Itoa(replicas),
		})
	}
	return nil
}

// after the first-replace,if the total-replicas is not equal to policy.spec.replicas,will assign again according to the weight, until they are equal
func (r *Reconciler) replaceReplicasByWeight(policy *v1alpha1.MultiClusterResourceSchedulePolicy, bindingResourceCluster *v1alpha1.MultiClusterResourceBindingResource, diffReplicas int, sortPolicy *controllerCommon.SortPolicy) error {
	var (
		fillNum        int
		addNum         int
		policyMaxOrMin int
		toMaxOrMin     int
	)
	switch {
	// if calculated replicas<policy.spec.repicas
	case diffReplicas > 0:
		fillNum = diffReplicas / len(policy.Spec.Policy)
		addNum = diffReplicas % len(policy.Spec.Policy)
	// if calculated replicas>policy.spec.repicas
	case diffReplicas < 0:
		fillNum = -diffReplicas / len(policy.Spec.Policy)
		addNum = -diffReplicas % len(policy.Spec.Policy)
	}

	for j, policyInstance := range policy.Spec.Policy {
		replicas, err := strconv.Atoi(bindingResourceCluster.Clusters[j].Override[0].Value)
		if err != nil {
			return err
		}
		switch {
		case diffReplicas > 0:
			policyMaxOrMin = policyInstance.Max
			toMaxOrMin = policyMaxOrMin - replicas
		case diffReplicas < 0:
			policyMaxOrMin = policyInstance.Min
			toMaxOrMin = replicas - policyMaxOrMin
		}
		if toMaxOrMin < fillNum {
			bindingResourceCluster.Clusters[j].Override[0].Value = strconv.Itoa(policyMaxOrMin)
			addNum += fillNum - toMaxOrMin
		} else {
			switch {
			case diffReplicas > 0:
				bindingResourceCluster.Clusters[j].Override[0].Value = strconv.Itoa(replicas + fillNum)
			case diffReplicas < 0:
				bindingResourceCluster.Clusters[j].Override[0].Value = strconv.Itoa(replicas - fillNum)
			}
		}

	}

	for addNum != 0 {
		for j, policyInstance := range policy.Spec.Policy {
			switch {
			case diffReplicas > 0:
				policyMaxOrMin = policyInstance.Max
			case diffReplicas < 0:
				policyMaxOrMin = policyInstance.Min
			}
			if policyInstance.Weight == sortPolicy.SortPolicyList[sortPolicy.SortPolicyListIndex].Weight && policyInstance.Name == sortPolicy.SortPolicyList[sortPolicy.SortPolicyListIndex].Name {
				replicas, err := strconv.Atoi(bindingResourceCluster.Clusters[j].Override[0].Value)
				if err != nil {
					return err
				}
				switch {
				case diffReplicas > 0:
					policyMaxOrMin = policyInstance.Max
					if replicas < policyMaxOrMin {
						bindingResourceCluster.Clusters[j].Override[0].Value = strconv.Itoa(replicas + 1)
						addNum = addNum - 1
						if sortPolicy.SortPolicyListIndex == len(sortPolicy.SortPolicyList)-1 {
							sortPolicy.SortPolicyListIndex = 0
						} else {
							sortPolicy.SortPolicyListIndex = sortPolicy.SortPolicyListIndex + 1
						}
					} else {
						sortPolicy.SortPolicyList = append(sortPolicy.SortPolicyList[:sortPolicy.SortPolicyListIndex], sortPolicy.SortPolicyList[sortPolicy.SortPolicyListIndex+1:]...)
						if sortPolicy.SortPolicyListIndex == len(sortPolicy.SortPolicyList) {
							sortPolicy.SortPolicyListIndex = 0
						}
						if len(sortPolicy.SortPolicyList) == 0 {
							return err
						}
					}
				case diffReplicas < 0:
					policyMaxOrMin = policyInstance.Min
					if replicas > policyMaxOrMin {
						bindingResourceCluster.Clusters[j].Override[0].Value = strconv.Itoa(replicas - 1)
						addNum = addNum - 1
						if sortPolicy.SortPolicyListIndex == len(sortPolicy.SortPolicyList)-1 {
							sortPolicy.SortPolicyListIndex = 0
						} else {
							sortPolicy.SortPolicyListIndex = sortPolicy.SortPolicyListIndex + 1
						}
					} else {
						sortPolicy.SortPolicyList = append(sortPolicy.SortPolicyList[:sortPolicy.SortPolicyListIndex], sortPolicy.SortPolicyList[sortPolicy.SortPolicyListIndex+1:]...)
						if sortPolicy.SortPolicyListIndex == len(sortPolicy.SortPolicyList) {
							sortPolicy.SortPolicyListIndex = 0
						}
						if len(sortPolicy.SortPolicyList) == 0 {
							return err
						}
					}
				}
			}
		}
	}

	return nil
}

func (r *Reconciler) generateBindingByClusterRole(ctx context.Context, policy *v1alpha1.MultiClusterResourceSchedulePolicy, clusterSet *v1alpha1.ClusterSet, failIndex []int, unavailableFailoverClusters []string) (*v1alpha1.MultiClusterResourceBinding, error) {
	binding := r.addBindingMeta(policy)
	clusterList := r.getClusterListByClusterSet(ctx, clusterSet)
	totalWeight, diff := r.calculateWeightByRole(policy, clusterSet)
	for i, resourceInstance := range policy.Spec.Resources {
		diffReplicas := diff
		binding.Spec.Resources = append(binding.Spec.Resources, v1alpha1.MultiClusterResourceBindingResource{Name: resourceInstance.Name})
		indexModel := controllerCommon.FailoverPolicyIndex{}
		sortPolicy := r.doSortPolicyList(policy)
		tempModel := &controllerCommon.FirstReplaceReplicasModel{
			ResourceName: resourceInstance.Name,
			TotalWeight:  totalWeight,
			DiffReplicas: diffReplicas,
		}
		for j, clusterName := range clusterList {
			if failIndex != nil {
				if indexModel.DoneIndex < len(failIndex) && j == failIndex[indexModel.DoneIndex] {
					err := r.doFailoverPolicy(ctx, &indexModel, policy.Spec.FailoverPolicy, &binding.Spec.Resources[i], unavailableFailoverClusters)
					if err != nil {
						return nil, err
					}
				} else {
					binding.Spec.Resources[i].Clusters = append(binding.Spec.Resources[i].Clusters, v1alpha1.MultiClusterResourceBindingCluster{Name: clusterName})
				}
			} else {
				binding.Spec.Resources[i].Clusters = append(binding.Spec.Resources[i].Clusters, v1alpha1.MultiClusterResourceBindingCluster{Name: clusterName})
			}

			err := r.firstReplaceReplicasByClusterRole(ctx, policy, &binding.Spec.Resources[i].Clusters[j], tempModel, clusterSet, clusterName)
			err = r.checkNamespaceMapping(ctx, policy, clusterName, &binding.Spec.Resources[i].Clusters[j])
			if err != nil {
				return nil, err
			}
		}
		if tempModel.DiffReplicas != 0 {
			err := r.replaceReplicasByClusterRole(ctx, policy, &binding.Spec.Resources[i], tempModel.DiffReplicas, sortPolicy, clusterSet)
			if err != nil {
				return nil, err
			}
		}
	}
	return binding, nil
}

func (r *Reconciler) firstReplaceReplicasByClusterRole(ctx context.Context, policy *v1alpha1.MultiClusterResourceSchedulePolicy, bindingResourceCluster *v1alpha1.MultiClusterResourceBindingCluster, model *controllerCommon.FirstReplaceReplicasModel, clusterSet *v1alpha1.ClusterSet, clusterName string) error {
	resource := &v1alpha1.MultiClusterResource{}
	resourceNamespacedName := types.NamespacedName{
		Name:      model.ResourceName,
		Namespace: policy.Namespace,
	}
	err := r.Client.Get(ctx, resourceNamespacedName, resource)
	if err != nil {
		return err
	}
	if len(resource.Spec.ReplicasField) > 0 {
		var replicas int
		var policyInstance v1alpha1.SchedulePolicy
		for _, cluster := range clusterSet.Spec.Clusters {
			if cluster.Name == clusterName {
				for _, instance := range policy.Spec.Policy {
					if instance.Role == cluster.Role {
						replicas = int((float64(policy.Spec.Replicas) / float64(model.TotalWeight)) * float64(instance.Weight))

						policyInstance = instance
						break
					}
				}
				break
			}
		}
		if replicas < policyInstance.Min {
			model.DiffReplicas -= policyInstance.Min - replicas
			replicas = policyInstance.Min

		}
		if replicas > policyInstance.Max {
			model.DiffReplicas += replicas - policyInstance.Max
			replicas = policyInstance.Max

		}

		bindingResourceCluster.Override = append(bindingResourceCluster.Override, apicommon.JSONPatch{
			Path:  resource.Spec.ReplicasField,
			Op:    pkgcommon.BindingOpReplace,
			Value: strconv.Itoa(replicas),
		})
	}
	return nil
}

func (r *Reconciler) replaceReplicasByClusterRole(ctx context.Context, policy *v1alpha1.MultiClusterResourceSchedulePolicy, bindingResourceCluster *v1alpha1.MultiClusterResourceBindingResource, diffReplicas int, sortPolicy *controllerCommon.SortPolicy, clusterSet *v1alpha1.ClusterSet) error {
	clusterList := r.getClusterListByClusterSet(ctx, clusterSet)
	sortPolicy.SortPolicyListIndex = -1
	var (
		fillNum        int
		addNum         int
		policyMaxOrMin int
		toMaxOrMin     int
	)
	switch {
	case diffReplicas > 0:
		fillNum = diffReplicas / len(clusterList)
		addNum = diffReplicas % len(clusterList)
	case diffReplicas < 0:
		fillNum = -(diffReplicas / len(clusterList))
		addNum = -(diffReplicas % len(clusterList))
	}

	for j, clusterName := range clusterList {
		replicas, err := strconv.Atoi(bindingResourceCluster.Clusters[j].Override[0].Value)
		if err != nil {
			return err
		}
		var policyInstance v1alpha1.SchedulePolicy
		for _, cluster := range clusterSet.Spec.Clusters {
			if cluster.Name == clusterName {
				for _, instance := range policy.Spec.Policy {
					if instance.Role == cluster.Role {
						policyInstance = instance
						break
					}
				}
				break
			}
		}
		switch {
		case diffReplicas > 0:
			policyMaxOrMin = policyInstance.Max
			toMaxOrMin = policyMaxOrMin - replicas
		case diffReplicas < 0:
			policyMaxOrMin = policyInstance.Min
			toMaxOrMin = replicas - policyMaxOrMin
		}

		if toMaxOrMin < fillNum {
			bindingResourceCluster.Clusters[j].Override[0].Value = strconv.Itoa(policyMaxOrMin)
			addNum += fillNum - toMaxOrMin
		} else {
			switch {
			case diffReplicas > 0:
				bindingResourceCluster.Clusters[j].Override[0].Value = strconv.Itoa(replicas + fillNum)
			case diffReplicas < 0:
				bindingResourceCluster.Clusters[j].Override[0].Value = strconv.Itoa(replicas - fillNum)
			}
		}
	}
	for addNum != 0 {
		if sortPolicy.SortPolicyListIndex == len(sortPolicy.SortPolicyList)-1 {
			sortPolicy.SortPolicyListIndex = 0
		} else {
			sortPolicy.SortPolicyListIndex += 1
		}

		for j, cluster := range clusterSet.Spec.Clusters {
			if addNum == 0 {
				break
			}
			var policyInstance v1alpha1.SchedulePolicy
			for _, instance := range policy.Spec.Policy {
				if instance.Role == cluster.Role {
					policyInstance = instance
					break
				}
			}

			if policyInstance.Role == sortPolicy.SortPolicyList[sortPolicy.SortPolicyListIndex].Role {
				replicas, err := strconv.Atoi(bindingResourceCluster.Clusters[j].Override[0].Value)
				if err != nil {
					return err
				}
				switch {
				case diffReplicas > 0:
					policyMaxOrMin = policyInstance.Max
					if replicas < policyMaxOrMin {
						bindingResourceCluster.Clusters[j].Override[0].Value = strconv.Itoa(replicas + 1)
						addNum = addNum - 1
					} else {
						sortPolicy.SortPolicyList = append(sortPolicy.SortPolicyList[:sortPolicy.SortPolicyListIndex], sortPolicy.SortPolicyList[sortPolicy.SortPolicyListIndex+1:]...)
						if len(sortPolicy.SortPolicyList) == 0 {
							return err
						}
						if sortPolicy.SortPolicyListIndex == len(sortPolicy.SortPolicyList) {
							sortPolicy.SortPolicyListIndex = -1
							break
						} else {
							continue
						}
					}
				case diffReplicas < 0:
					policyMaxOrMin = policyInstance.Min
					if replicas > policyMaxOrMin {
						bindingResourceCluster.Clusters[j].Override[0].Value = strconv.Itoa(replicas - 1)
						addNum = addNum - 1
					} else {
						sortPolicy.SortPolicyList = append(sortPolicy.SortPolicyList[:sortPolicy.SortPolicyListIndex], sortPolicy.SortPolicyList[sortPolicy.SortPolicyListIndex+1:]...)
						if len(sortPolicy.SortPolicyList) == 0 {
							return err
						}
						if sortPolicy.SortPolicyListIndex == len(sortPolicy.SortPolicyList) {
							sortPolicy.SortPolicyListIndex = -1
							break
						} else {
							continue
						}
					}
				}
			}
		}
	}

	return nil
}

func (r *Reconciler) generateBindingByClusterSelector(ctx context.Context, policy *v1alpha1.MultiClusterResourceSchedulePolicy, clusterSet *v1alpha1.ClusterSet, failIndex []int, unavailableFailoverClusters []string) (*v1alpha1.MultiClusterResourceBinding, error) {
	binding := r.addBindingMeta(policy)
	clusterList := r.getClusterListByClusterSet(ctx, clusterSet)
	for i, resourceInstance := range policy.Spec.Resources {
		binding.Spec.Resources = append(binding.Spec.Resources, v1alpha1.MultiClusterResourceBindingResource{Name: resourceInstance.Name})
		indexModel := controllerCommon.FailoverPolicyIndex{}
		for j, clusterName := range clusterList {
			if failIndex != nil {

				if indexModel.DoneIndex < len(failIndex) && j == failIndex[indexModel.DoneIndex] {

					err := r.doFailoverPolicy(ctx, &indexModel, policy.Spec.FailoverPolicy, &binding.Spec.Resources[i], unavailableFailoverClusters)
					if err != nil {
						return nil, err
					}
				} else {
					binding.Spec.Resources[i].Clusters = append(binding.Spec.Resources[i].Clusters, v1alpha1.MultiClusterResourceBindingCluster{Name: clusterName})
				}
			} else {
				binding.Spec.Resources[i].Clusters = append(binding.Spec.Resources[i].Clusters, v1alpha1.MultiClusterResourceBindingCluster{Name: clusterName})
			}
			err := r.replaceResourceReplicasField(ctx, policy, resourceInstance.Name, &binding.Spec.Resources[i].Clusters[j])
			if err != nil {
				return nil, err
			}

			err = r.checkNamespaceMapping(ctx, policy, clusterName, &binding.Spec.Resources[i].Clusters[j])
			if err != nil {
				return nil, err
			}
		}
	}
	return binding, nil
}

// calculate number of availabel failover clusters and unavailabel failover clusters
func (r *Reconciler) failoverPolicyCheck(ctx context.Context, policy *v1alpha1.MultiClusterResourceSchedulePolicy) (int, []string) {

	failoverCount := 0
	var unavailableClusters []string
	for _, instance := range policy.Spec.FailoverPolicy {
		if instance.Type == apicommon.ClusterTypeClusters {
			err := r.checkCluster(ctx, instance.Name)
			if err != nil {
				unavailableClusters = append(unavailableClusters, instance.Name)
				continue
			}
			failoverCount = failoverCount + 1
		} else if instance.Type == apicommon.ClusterTypeClusterSet {

			clusterSet := &v1alpha1.ClusterSet{}
			clusterSetNamespacedName := types.NamespacedName{
				Name: instance.Name,
			}
			if err := r.Client.Get(ctx, clusterSetNamespacedName, clusterSet); err != nil {

				unavailableClusters = append(unavailableClusters, instance.Name)
				continue
			}
			clusterList := r.getClusterListByClusterSet(ctx, clusterSet)
			for _, cluster := range clusterList {
				err := r.checkCluster(ctx, cluster)
				if err != nil {
					unavailableClusters = append(unavailableClusters, cluster)
					continue
				}
				failoverCount = failoverCount + 1
			}
		}
	}
	return failoverCount, unavailableClusters
}

func (r *Reconciler) addBindingMeta(policy *v1alpha1.MultiClusterResourceSchedulePolicy) *v1alpha1.MultiClusterResourceBinding {
	binding := &v1alpha1.MultiClusterResourceBinding{}
	bindingName := pkgcommon.Scheduler + "-" + policy.Name
	binding.Name = bindingName
	binding.Namespace = policy.Namespace
	owner := metav1.NewControllerRef(policy, v1alpha1.MultiClusterResourceSchedulePolicyGroupVersionKind)
	binding.SetOwnerReferences([]metav1.OwnerReference{*owner})
	return binding
}

// do namespace mapping if it exists
func (r *Reconciler) checkNamespaceMapping(ctx context.Context, policy *v1alpha1.MultiClusterResourceSchedulePolicy, policyName string, bindingResourceCluster *v1alpha1.MultiClusterResourceBindingCluster) error {
	namespace := &corev1.Namespace{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: policy.Namespace}, namespace)
	if err != nil {
		return err
	}
	labels := namespace.Labels
	mappingK, err := controllerCommon.GenerateLabelKey(policyName, policy.Namespace)
	if err != nil {
		return err
	}
	if value, ok := labels[mappingK]; ok {
		bindingResourceCluster.Override = append(bindingResourceCluster.Override, apicommon.JSONPatch{
			Path:  pkgcommon.BindingPathNamespace,
			Op:    pkgcommon.BindingOpReplace,
			Value: value,
		})

	}
	return nil
}

func (r *Reconciler) replaceResourceReplicasField(ctx context.Context, policy *v1alpha1.MultiClusterResourceSchedulePolicy, resourceName string, bindingResourceCluster *v1alpha1.MultiClusterResourceBindingCluster) error {
	resource := &v1alpha1.MultiClusterResource{}
	resourceNamespacedName := types.NamespacedName{
		Name:      resourceName,
		Namespace: policy.Namespace,
	}
	err := r.Client.Get(ctx, resourceNamespacedName, resource)
	if err != nil {
		return err
	}
	if len(resource.Spec.ReplicasField) > 0 {
		bindingResourceCluster.Override = append(bindingResourceCluster.Override, apicommon.JSONPatch{
			Path:  resource.Spec.ReplicasField,
			Op:    pkgcommon.BindingOpReplace,
			Value: strconv.Itoa(policy.Spec.Replicas),
		})

	}
	return nil
}

func (r *Reconciler) calculateWeight(policy *v1alpha1.MultiClusterResourceSchedulePolicy) (int, int) {
	var (
		totalWeight  = 0
		diffReplicas = 0
		nowReplicas  = 0
	)

	for _, instance := range policy.Spec.Policy {
		totalWeight = totalWeight + instance.Weight
	}
	weight := float64(policy.Spec.Replicas) / float64(totalWeight)
	for _, instance := range policy.Spec.Policy {
		nowReplicas = nowReplicas + int(weight*float64(instance.Weight))
	}
	if nowReplicas != policy.Spec.Replicas {
		diffReplicas = policy.Spec.Replicas - nowReplicas
	}

	return totalWeight, diffReplicas
}

func (r *Reconciler) calculateWeightByRole(policy *v1alpha1.MultiClusterResourceSchedulePolicy, clusterSet *v1alpha1.ClusterSet) (int, int) {
	var (
		totalWeight  = 0
		diffReplicas = 0
		nowReplicas  = 0
	)
	for _, instance := range policy.Spec.Policy {
		for _, cluster := range clusterSet.Spec.Clusters {
			if cluster.Role == instance.Role {
				totalWeight = totalWeight + instance.Weight
			}
		}
	}
	weight := float64(policy.Spec.Replicas) / float64(totalWeight)
	for _, instance := range policy.Spec.Policy {
		for _, cluster := range clusterSet.Spec.Clusters {
			if cluster.Role == instance.Role {
				nowReplicas = nowReplicas + int(weight*float64(instance.Weight))
			}
		}
	}
	if nowReplicas != policy.Spec.Replicas {
		diffReplicas = policy.Spec.Replicas - nowReplicas
	}
	return totalWeight, diffReplicas

}

// get clusters by clusterset
func (r *Reconciler) getClusterListByClusterSet(ctx context.Context, clusterSet *v1alpha1.ClusterSet) []string {
	var clusterList []string
	if len(clusterSet.Spec.Clusters) > 0 {
		for _, clusterName := range clusterSet.Spec.Clusters {
			clusterList = append(clusterList, clusterName.Name)
		}
	} else if len(clusterSet.Spec.Selector.Labels) > 0 {
		selector := labels.SelectorFromSet(clusterSet.Spec.Selector.Labels)
		list := &v1alpha1.ClusterList{}
		err := r.Client.List(ctx, list, &client.ListOptions{
			LabelSelector: selector,
		})
		if err != nil {
			return clusterList
		}
		for _, item := range list.Items {
			clusterList = append(clusterList, item.Name)
		}
	}
	return clusterList
}

// do failover policy
func (r *Reconciler) doFailoverPolicy(ctx context.Context, index *controllerCommon.FailoverPolicyIndex, failoverPolicy []v1alpha1.ScheduleFailoverPolicy, bindingCluster *v1alpha1.MultiClusterResourceBindingResource, unavailableFailoverClusters []string) error {
	for {
		if failoverPolicy[index.FailoverIndex].Type == apicommon.ClusterTypeClusters {
			// all failover clusters are availabel,or current failover cluster is availabel
			if len(unavailableFailoverClusters) == 0 || failoverPolicy[index.FailoverIndex].Name != unavailableFailoverClusters[index.UnavailableFailoverClusterIndex] {
				bindingCluster.Clusters = append(bindingCluster.Clusters, v1alpha1.MultiClusterResourceBindingCluster{Name: failoverPolicy[index.FailoverIndex].Name})
				// the cluster currently requiring failover is done
				index.DoneIndex++
				// to next failover cluster
				index.FailoverIndex++
				break
			} else {
				index.FailoverIndex++
				if index.UnavailableFailoverClusterIndex < len(unavailableFailoverClusters)-1 {
					index.UnavailableFailoverClusterIndex++
				}

			}

		} else if failoverPolicy[index.FailoverIndex].Type == apicommon.ClusterTypeClusterSet {
			clusterSet := &v1alpha1.ClusterSet{}
			err := r.Client.Get(ctx, types.NamespacedName{Name: failoverPolicy[index.FailoverIndex].Name}, clusterSet)
			if err != nil {
				return err
			}
			clusterList := r.getClusterListByClusterSet(ctx, clusterSet)
			if len(unavailableFailoverClusters) == 0 || clusterList[index.ClusterSetIndex] != unavailableFailoverClusters[index.UnavailableFailoverClusterIndex] {
				bindingCluster.Clusters = append(bindingCluster.Clusters, v1alpha1.MultiClusterResourceBindingCluster{Name: clusterList[index.ClusterSetIndex]})
				index.DoneIndex++
				if index.ClusterSetIndex < len(clusterList)-1 {
					index.ClusterSetIndex++
				} else {
					// reset clusterset
					index.ClusterSetIndex = 0
					index.FailoverIndex++
				}
				break
			} else {
				index.ClusterSetIndex++
				if index.UnavailableFailoverClusterIndex < len(unavailableFailoverClusters)-1 {
					index.UnavailableFailoverClusterIndex++
				}
			}

		}
	}
	return nil
}

func (r *Reconciler) updateLastModifyTime(ctx context.Context, policy *v1alpha1.MultiClusterResourceSchedulePolicy) error {
	modifyTime := metav1.Time{Time: time.Now()}
	policy.Status.Schedule.LastModifyTime = &modifyTime
	err := r.Client.Status().Update(ctx, policy)
	if err != nil {
		return err
	}
	return nil

}
func (r *Reconciler) updateLastScheduleTime(ctx context.Context, policy *v1alpha1.MultiClusterResourceSchedulePolicy) error {
	scheduleTime := metav1.Time{Time: time.Now()}
	policy.Status.Schedule.LastScheduleTime = &scheduleTime
	err := r.Client.Status().Update(ctx, policy)
	if err != nil {
		return err
	}
	return nil
}

func (r *Reconciler) compareBinding(ctx context.Context, binding *v1alpha1.MultiClusterResourceBinding) bool {
	previousBinding := &v1alpha1.MultiClusterResourceBinding{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace}, previousBinding)
	if err != nil {
		return false
	}
	if !reflect.DeepEqual(previousBinding.Spec, binding.Spec) {
		return false
	}
	return true
}

func (r *Reconciler) createOrUpdateBinding(ctx context.Context, binding *v1alpha1.MultiClusterResourceBinding) error {
	instance := &v1alpha1.MultiClusterResourceBinding{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: binding.Name, Namespace: binding.Namespace}, instance)
	if errors.IsNotFound(err) {
		err = r.Client.Create(ctx, binding)
		if err != nil {
			r.log.Error(err, "fail to create resource binding")
			return err
		}
		return nil
	}
	if err != nil {
		return err
	}
	// compare binding spec
	if reflect.DeepEqual(instance.Spec, binding.Spec) {
		return nil
	}
	instance.Spec = binding.Spec
	err = r.Client.Update(ctx, instance)
	if err != nil {
		r.log.Error(err, "fail to update resource binding")
		return err
	}
	return nil
}

func (r *Reconciler) doSortPolicyList(policy *v1alpha1.MultiClusterResourceSchedulePolicy) *controllerCommon.SortPolicy {

	sortPolicyList := append(policy.Spec.Policy[:0:0], policy.Spec.Policy...)
	sort.Slice(sortPolicyList, func(i, j int) bool {
		return sortPolicyList[i].Weight > sortPolicyList[j].Weight
	})
	sortPolicyListIndex := 0
	sortPolicy := &controllerCommon.SortPolicy{SortPolicyList: sortPolicyList, SortPolicyListIndex: sortPolicyListIndex}
	return sortPolicy
}

func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.MultiClusterResourceSchedulePolicy{}).
		Complete(r)
}

func Setup(mgr ctrl.Manager, controllerCommon controllerCommon.Args) error {
	reconciler := Reconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		log:    logf.Log.WithName("schedule_policy_controller"),
	}
	return reconciler.SetupWithManager(mgr)
}
