package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	"github.com/PrefectHQ/prefect-operator/internal/prefect"
)

var _ = Describe("PrefectWorkPool work queues", func() {
	var (
		ctx        context.Context
		reconciler *PrefectWorkPoolReconciler
		mockClient *prefect.MockClient
		workPool   *prefectiov1.PrefectWorkPool
	)

	BeforeEach(func() {
		ctx = context.Background()
		reconciler = &PrefectWorkPoolReconciler{}
		mockClient = prefect.NewMockClient()
		workPool = &prefectiov1.PrefectWorkPool{}
		workPool.Name = "example-pool"
	})

	It("creates a declared queue that does not exist yet", func() {
		workPool.Spec.WorkQueues = []prefectiov1.PrefectWorkQueue{
			{Name: "ingest", ConcurrencyLimit: new(int32(1))},
		}

		Expect(reconciler.syncWorkQueues(ctx, mockClient, workPool)).To(Succeed())

		queue, err := mockClient.GetWorkQueue(ctx, "example-pool", "ingest")
		Expect(err).NotTo(HaveOccurred())
		Expect(queue).NotTo(BeNil())
		Expect(*queue.ConcurrencyLimit).To(Equal(int32(1)))
	})

	It("updates a queue whose concurrency limit has drifted", func() {
		mockClient.SeedWorkQueue("example-pool", &prefect.WorkQueue{
			Name: "transform", ConcurrencyLimit: new(int32(9)),
		})
		workPool.Spec.WorkQueues = []prefectiov1.PrefectWorkQueue{
			{Name: "transform", ConcurrencyLimit: new(int32(2))},
		}

		Expect(reconciler.syncWorkQueues(ctx, mockClient, workPool)).To(Succeed())

		queue, err := mockClient.GetWorkQueue(ctx, "example-pool", "transform")
		Expect(err).NotTo(HaveOccurred())
		Expect(*queue.ConcurrencyLimit).To(Equal(int32(2)))
		Expect(mockClient.UpdateWorkQueueCalls).To(Equal(1))
	})

	It("does not PATCH a queue that already matches", func() {
		mockClient.SeedWorkQueue("example-pool", &prefect.WorkQueue{
			Name: "transform", ConcurrencyLimit: new(int32(2)), Priority: new(int32(3)),
		})
		workPool.Spec.WorkQueues = []prefectiov1.PrefectWorkQueue{
			{Name: "transform", ConcurrencyLimit: new(int32(2)), Priority: new(int32(3))},
		}

		Expect(reconciler.syncWorkQueues(ctx, mockClient, workPool)).To(Succeed())
		Expect(mockClient.UpdateWorkQueueCalls).To(Equal(0))
	})

	It("ignores fields the spec leaves unset", func() {
		// Queue is paused and described remotely; the spec only pins the limit,
		// so those remote values must not count as drift.
		mockClient.SeedWorkQueue("example-pool", &prefect.WorkQueue{
			Name:             "report",
			ConcurrencyLimit: new(int32(1)),
			IsPaused:         new(true),
			Description:      new("set elsewhere"),
		})
		workPool.Spec.WorkQueues = []prefectiov1.PrefectWorkQueue{
			{Name: "report", ConcurrencyLimit: new(int32(1))},
		}

		Expect(reconciler.syncWorkQueues(ctx, mockClient, workPool)).To(Succeed())
		Expect(mockClient.UpdateWorkQueueCalls).To(Equal(0))
	})

	It("leaves undeclared queues alone", func() {
		mockClient.SeedWorkQueue("example-pool", &prefect.WorkQueue{
			Name: "unmanaged", ConcurrencyLimit: new(int32(7)),
		})
		workPool.Spec.WorkQueues = []prefectiov1.PrefectWorkQueue{
			{Name: "ingest", ConcurrencyLimit: new(int32(1))},
		}

		Expect(reconciler.syncWorkQueues(ctx, mockClient, workPool)).To(Succeed())

		queue, err := mockClient.GetWorkQueue(ctx, "example-pool", "unmanaged")
		Expect(err).NotTo(HaveOccurred())
		Expect(queue).NotTo(BeNil())
		Expect(*queue.ConcurrencyLimit).To(Equal(int32(7)))
	})

	It("is a no-op when no queues are declared", func() {
		Expect(reconciler.syncWorkQueues(ctx, mockClient, workPool)).To(Succeed())
		Expect(mockClient.UpdateWorkQueueCalls).To(Equal(0))
	})
})
