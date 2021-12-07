package resource_binding_controller_test

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"time"

	"harmonycloud.cn/multi-cluster-manager/pkg/apis/multicluster/common"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/labels"

	"harmonycloud.cn/multi-cluster-manager/pkg/util/sliceutil"

	"github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	. "github.com/onsi/ginkgo"
	"harmonycloud.cn/multi-cluster-manager/pkg/apis/multicluster/v1alpha1"
	"harmonycloud.cn/multi-cluster-manager/pkg/client/clientset/versioned/fake"
	informers "harmonycloud.cn/multi-cluster-manager/pkg/client/informers/externalversions"
	. "harmonycloud.cn/multi-cluster-manager/pkg/controller/resource_binding_controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var _ = Describe("ResourceBindingController", func() {
	// 1、binding增加时，会增加特定的finalizer
	// 2、binding更改时，只有binding的spec变更才会被监听，此时需要同步clusterResource
	// 3、binding被删除时，binding需要删除相关的clusterResource，再删除自身的特定的finalizer
	// 4、相关联的multiClusterResource的spec被修改时，会重新同步clusterResource
	// 5、相关联的multiClusterResource被删除时，不会重新同步clusterResource
	// 6、相关联的clusterResource状态变更时，binding会监听到再变更自己的status
	// 7、相关联的clusterResource被删除时，会重新同步clusterResource

	var (
		managerClient         *fake.Clientset
		informerFactory       informers.SharedInformerFactory
		controller            *ResourceBindingController
		resourceBinding       *v1alpha1.MultiClusterResourceBinding
		multiClusterResource  *v1alpha1.MultiClusterResource
		resourceJsonString    string
		newResourceJsonString string
		resourceGvk           *schema.GroupVersionKind
		clusterNameList       []string
	)
	//var testLog = logf.Log.WithName("bindingControllerTest")
	//
	BeforeEach(func() {
		//
		client := fake.NewSimpleClientset()
		managerClient = client
		//
		informer := informers.NewSharedInformerFactory(client, 0)
		informerFactory = informer

		config, err := rest.InClusterConfig()
		if err != nil {
			k8sConfig := flag.String("k8sconfig", "/Users/chenkun/Desktop/k8s/config-238", "kubernetes auth config")
			config, _ = clientcmd.BuildConfigFromFlags("", *k8sConfig)
		}
		// new controller
		controller = NewResourceBindingController(managerClient,
			informerFactory.Multicluster().V1alpha1().MultiClusterResourceBindings(),
			informerFactory.Multicluster().V1alpha1().MultiClusterResources(),
			informerFactory.Multicluster().V1alpha1().ClusterResources(),
			config)

		controller.Run(context.Background(), 2)
	})

	JustBeforeEach(func() {
		resourceBinding = &v1alpha1.MultiClusterResourceBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "resourceBinding",
				Namespace: ManagerNamespace,
			},
			Spec: v1alpha1.MultiClusterResourceBindingSpec{
				Resources: []v1alpha1.MultiClusterResourceBindingResource{},
			},
		}

		multiClusterResource = &v1alpha1.MultiClusterResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "multiClusterResource",
				Namespace: ManagerNamespace,
			},
			Spec: v1alpha1.MultiClusterResourceSpec{
				Resource: &runtime.RawExtension{
					Raw: []byte(resourceJsonString),
				},
				ResourceRef: resourceGvk,
			},
		}

		By(fmt.Sprintf("create multiClusterResource(%s)", multiClusterResource.Name), func() {
			_, err := managerClient.MulticlusterV1alpha1().MultiClusterResources(ManagerNamespace).Create(context.TODO(), multiClusterResource, metav1.CreateOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})

		By(fmt.Sprintf("create binding(%s)", resourceBinding.Name), func() {
			_, err := managerClient.MulticlusterV1alpha1().MultiClusterResourceBindings(ManagerNamespace).Create(context.TODO(), resourceBinding, metav1.CreateOptions{})
			gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
		})
	}, 5)

	//
	Describe("controller event test", func() {
		Context("create binding", func() {
			It(fmt.Sprintf("create binding：%s", resourceBinding.Name), func() {
				// check binding finalizers
				By(fmt.Sprintf("check binding(%s) finalizers", resourceBinding.Name), func() {
					err := func() error {
						binding, err := informerFactory.Multicluster().V1alpha1().MultiClusterResourceBindings().Lister().MultiClusterResourceBindings(ManagerNamespace).Get(resourceBinding.Name)
						if err != nil {
							return err
						}
						bindingFinalizers := binding.GetFinalizers()
						if len(bindingFinalizers) <= 0 && !sliceutil.ContainsString(bindingFinalizers, FinalizerName) {
							return errors.New("add finalizers fail")
						}
						return nil
					}()
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				})
				// update binding spec and check
				By(fmt.Sprintf("update binding(%s) spec and check ClusterResourceList", resourceBinding.Name), func() {
					err := func() error {
						binding, err := informerFactory.Multicluster().V1alpha1().MultiClusterResourceBindings().Lister().MultiClusterResourceBindings(ManagerNamespace).Get(resourceBinding.Name)
						if err != nil {
							return err
						}
						var clusters []v1alpha1.MultiClusterResourceBindingCluster
						for _, item := range clusterNameList {
							clusters = append(clusters, v1alpha1.MultiClusterResourceBindingCluster{
								Name: item,
							})
						}
						binding.Spec.Resources = append(binding.Spec.Resources, v1alpha1.MultiClusterResourceBindingResource{
							Name:     multiClusterResource.Name,
							Clusters: clusters,
						})

						_, err = managerClient.MulticlusterV1alpha1().MultiClusterResourceBindings(ManagerNamespace).Update(context.TODO(), binding, metav1.UpdateOptions{})
						if err != nil {
							return err
						}
						// Let the bullet fly for a while
						time.Sleep(10 * time.Second)

						labelString := ResourceBindingLabelName + "=" + resourceBinding.Name
						selector, err := labels.Parse(labelString)
						if err != nil {
							return err
						}
						clusterResourceList, err := informerFactory.Multicluster().V1alpha1().ClusterResources().Lister().List(selector)
						if err != nil {
							return err
						}
						if len(clusterResourceList) <= 0 {
							return errors.New("find clusterResource fail")
						}
						return nil
					}()
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				})

				// update multiClusterResource spec and check
				By(fmt.Sprintf("update multiClusterResource(%s) and check ClusterResourceList", multiClusterResource.Name), func() {
					err := func() error {
						multiCR, err := informerFactory.Multicluster().V1alpha1().MultiClusterResources().Lister().MultiClusterResources(ManagerNamespace).Get(multiClusterResource.Name)
						if err != nil {
							return err
						}
						multiCR.Spec.Resource = &runtime.RawExtension{
							Raw: []byte(newResourceJsonString),
						}
						_, err = managerClient.MulticlusterV1alpha1().MultiClusterResources(ManagerNamespace).Update(context.TODO(), multiCR, metav1.UpdateOptions{})
						if err != nil {
							return err
						}
						// Let the bullet fly for a while
						time.Sleep(20 * time.Second)
						//
						labelString := ResourceBindingLabelName + "=" + resourceBinding.Name
						selector, err := labels.Parse(labelString)
						if err != nil {
							return err
						}
						clusterResourceList, err := informerFactory.Multicluster().V1alpha1().ClusterResources().Lister().List(selector)
						if err != nil {
							return err
						}
						if len(clusterResourceList) <= 0 {
							return errors.New("clusterResourceList should not be empty")
						}
						for _, item := range clusterResourceList {
							if string(item.Spec.Resource.Raw) != newResourceJsonString {
								return errors.New("ClusterResource should be update to newResourceJsonString")
							}
						}
						return nil
					}()
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				})

				// update clusterResource status and check
				By(fmt.Sprintf("update clusterResource status and check binding status"), func() {
					err := func() error {
						labelString := ResourceBindingLabelName + "=" + resourceBinding.Name
						selector, err := labels.Parse(labelString)
						if err != nil {
							return err
						}
						clusterResourceList, err := informerFactory.Multicluster().V1alpha1().ClusterResources().Lister().List(selector)
						if err != nil {
							return err
						}
						completeString := "sync complete"
						for _, item := range clusterResourceList {
							item.Status = v1alpha1.ClusterResourceStatus{
								Phase:   common.Complete,
								Message: completeString,
							}
							_, err = managerClient.MulticlusterV1alpha1().ClusterResources(item.GetNamespace()).UpdateStatus(context.TODO(), item, metav1.UpdateOptions{})
							if err != nil {
								return err
							}
						}
						// Let the bullet fly for a while
						time.Sleep(10 * time.Second)
						// check
						binding, err := informerFactory.Multicluster().V1alpha1().MultiClusterResourceBindings().Lister().MultiClusterResourceBindings(ManagerNamespace).Get(resourceBinding.Name)
						if err != nil {
							return err
						}
						if binding.Status.ClusterStatus == nil {
							return errors.New("binding status should not be nil")
						}
						if !(binding.Status.ClusterStatus[0].Phase == common.Complete && binding.Status.ClusterStatus[0].Message == completeString) {
							return errors.New("binding status phase or message error")
						}
						return nil
					}()
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				})

				// delete clusterResource and check rebuild or not
				By(fmt.Sprintf("delete clusterResource and check rebuild or not"), func() {
					err := func() error {
						labelString := ResourceBindingLabelName + "=" + resourceBinding.Name
						selector, err := labels.Parse(labelString)
						if err != nil {
							return err
						}
						clusterResourceList, err := informerFactory.Multicluster().V1alpha1().ClusterResources().Lister().List(selector)
						if err != nil {
							return err
						}

						cr := clusterResourceList[0]

						err = managerClient.MulticlusterV1alpha1().ClusterResources(cr.Namespace).Delete(context.TODO(), cr.Name, metav1.DeleteOptions{})
						if err != nil {
							return err
						}

						time.Sleep(10 * time.Second)
						//
						_, err = informerFactory.Multicluster().V1alpha1().ClusterResources().Lister().ClusterResources(cr.Namespace).Get(cr.Name)
						if err != nil {
							if apierrors.IsNotFound(err) {
								return errors.New(fmt.Sprintf("clusterResource(%s) should alived", cr.Name))
							}
							return err
						}
						return nil
					}()
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				})

				// delete multiClusterResource and check binding
				By(fmt.Sprintf("delete multiClusterResource(%s) and check binding", multiClusterResource.Name), func() {
					err := func() error {
						multiCR, err := informerFactory.Multicluster().V1alpha1().MultiClusterResources().Lister().MultiClusterResources(ManagerNamespace).Get(multiClusterResource.Name)
						if err != nil {
							return err
						}
						err = managerClient.MulticlusterV1alpha1().MultiClusterResources(ManagerNamespace).Delete(context.TODO(), multiCR.Name, metav1.DeleteOptions{})
						if err != nil {
							return err
						}
						// Let the bullet fly for a while
						time.Sleep(10 * time.Second)

						_, err = informerFactory.Multicluster().V1alpha1().MultiClusterResourceBindings().Lister().MultiClusterResourceBindings(ManagerNamespace).Get(resourceBinding.Name)
						if err != nil {
							return err
						}

						return nil
					}()
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())
				})

				// delete binding and check binding
				By(fmt.Sprintf("delete binding(%s) and check binding", resourceBinding.Name), func() {
					err := func() error {
						err := managerClient.MulticlusterV1alpha1().MultiClusterResourceBindings(ManagerNamespace).Delete(context.TODO(), resourceBinding.Name, metav1.DeleteOptions{})
						if err != nil {
							return err
						}
						// Let the bullet fly for a while
						time.Sleep(10 * time.Second)
						//
						_, err = informerFactory.Multicluster().V1alpha1().MultiClusterResourceBindings().Lister().MultiClusterResourceBindings(ManagerNamespace).Get(resourceBinding.Name)
						if !apierrors.IsNotFound(err) {
							return errors.New(fmt.Sprintf("binding(%s) should be deleted", resourceBinding.Name))
						}
						labelString := ResourceBindingLabelName + "=" + resourceBinding.Name
						selector, err := labels.Parse(labelString)
						if err != nil {
							return err
						}
						clusterResourceList, _ := informerFactory.Multicluster().V1alpha1().ClusterResources().Lister().List(selector)
						if len(clusterResourceList) > 0 {
							return errors.New("clusterResourceList should be deleted")
						}
						return nil
					}()
					gomega.Expect(err).ShouldNot(gomega.HaveOccurred())

				})

			})

		})
	})

})
