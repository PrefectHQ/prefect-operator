package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
)

var _ = Describe("checkSharedState", func() {
	var reconciler *PrefectServerReconciler

	BeforeEach(func() {
		reconciler = &PrefectServerReconciler{}
	})

	sharedState := func(server *prefectiov1.PrefectServer) *metav1.Condition {
		reconciler.checkSharedState(server)
		return meta.FindStatusCondition(server.Status.Conditions, "SharedStateConfigured")
	}

	It("is True for a single-replica server", func() {
		condition := sharedState(&prefectiov1.PrefectServer{})
		Expect(condition).NotTo(BeNil())
		Expect(condition.Status).To(Equal(metav1.ConditionTrue))
		Expect(condition.Reason).To(Equal("SingleReplica"))
	})

	It("warns for replicas > 1 without redis", func() {
		condition := sharedState(&prefectiov1.PrefectServer{
			Spec: prefectiov1.PrefectServerSpec{Replicas: new(int32(3))},
		})
		Expect(condition).NotTo(BeNil())
		Expect(condition.Status).To(Equal(metav1.ConditionFalse))
		Expect(condition.Reason).To(Equal("InMemoryLeaseStorage"))
	})

	It("warns for replicas > 1 with redis but no lease storage", func() {
		condition := sharedState(&prefectiov1.PrefectServer{
			Spec: prefectiov1.PrefectServerSpec{
				Replicas: new(int32(3)),
				Redis:    &prefectiov1.RedisConfiguration{Host: new("redis.example.com")},
			},
		})
		Expect(condition).NotTo(BeNil())
		Expect(condition.Status).To(Equal(metav1.ConditionFalse))
		Expect(condition.Reason).To(Equal("InMemoryLeaseStorage"))
	})

	It("is True for replicas > 1 with redis lease storage", func() {
		condition := sharedState(&prefectiov1.PrefectServer{
			Spec: prefectiov1.PrefectServerSpec{
				Replicas: new(int32(3)),
				Redis: &prefectiov1.RedisConfiguration{
					Host:         new("redis.example.com"),
					LeaseStorage: new(true),
				},
			},
		})
		Expect(condition).NotTo(BeNil())
		Expect(condition.Status).To(Equal(metav1.ConditionTrue))
		Expect(condition.Reason).To(Equal("RedisLeaseStorage"))
	})
})
