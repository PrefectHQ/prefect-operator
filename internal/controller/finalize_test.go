package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("finalize", func() {
	const (
		finalizer      = "prefect.io/test-cleanup"
		otherFinalizer = "example.com/other"
	)

	var (
		ctx   context.Context
		name  types.NamespacedName
		calls int
	)

	// deleting creates a ConfigMap with finalizers, deletes it so that it has
	// a deletion timestamp, and returns it as read back.
	deleting := func(finalizers ...string) *corev1.ConfigMap {
		obj := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
			Name:       name.Name,
			Namespace:  name.Namespace,
			Finalizers: finalizers,
		}}
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
		Expect(k8sClient.Delete(ctx, obj)).To(Succeed())
		Expect(k8sClient.Get(ctx, name, obj)).To(Succeed())
		Expect(obj.DeletionTimestamp).NotTo(BeNil())
		return obj
	}

	expectGone := func() {
		err := k8sClient.Get(ctx, name, &corev1.ConfigMap{})
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "expected the ConfigMap to be deleted, got %v", err)
	}

	BeforeEach(func() {
		ctx = context.Background()
		calls = 0
		namespaceName := fmt.Sprintf("finalize-ns-%d", time.Now().UnixNano())
		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespaceName}})).To(Succeed())
		name = types.NamespacedName{Namespace: namespaceName, Name: "finalize-test"}
	})

	It("deletes from Prefect and then removes the finalizer", func() {
		obj := deleting(finalizer)

		result, err := finalize(ctx, k8sClient, obj, finalizer, func(context.Context) error {
			calls++
			return nil
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(result.IsZero()).To(BeTrue())
		Expect(calls).To(Equal(1))
		expectGone()
	})

	It("removes the finalizer when the Prefect delete fails", func() {
		obj := deleting(finalizer)

		_, err := finalize(ctx, k8sClient, obj, finalizer, func(context.Context) error {
			calls++
			return errors.New("prefect is down")
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(calls).To(Equal(1))
		expectGone()
	})

	It("removes the finalizer when there's nothing to delete in Prefect", func() {
		obj := deleting(finalizer)

		_, err := finalize(ctx, k8sClient, obj, finalizer, nil)

		Expect(err).NotTo(HaveOccurred())
		expectGone()
	})

	It("does nothing when the object doesn't have the finalizer", func() {
		obj := deleting(otherFinalizer)

		_, err := finalize(ctx, k8sClient, obj, finalizer, func(context.Context) error {
			calls++
			return nil
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(calls).To(BeZero())
		Expect(k8sClient.Get(ctx, name, obj)).To(Succeed())
		Expect(obj.Finalizers).To(ConsistOf(otherFinalizer))

		controllerutil.RemoveFinalizer(obj, otherFinalizer)
		Expect(k8sClient.Update(ctx, obj)).To(Succeed())
	})

	It("returns an error when it can't remove the finalizer", func() {
		obj := deleting(finalizer)
		stale := obj.DeepCopy()

		// Change the object so that an update from the stale copy conflicts.
		obj.Labels = map[string]string{"changed": "true"}
		Expect(k8sClient.Update(ctx, obj)).To(Succeed())

		_, err := finalize(ctx, k8sClient, stale, finalizer, nil)

		Expect(apierrors.IsConflict(err)).To(BeTrue(), "expected a conflict, got %v", err)
		Expect(k8sClient.Get(ctx, name, obj)).To(Succeed())
		Expect(obj.Finalizers).To(ConsistOf(finalizer))

		controllerutil.RemoveFinalizer(obj, finalizer)
		Expect(k8sClient.Update(ctx, obj)).To(Succeed())
	})
})
