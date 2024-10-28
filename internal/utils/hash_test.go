package utils

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func MigrationJobStub() *batchv1.Job {
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-migration-job",
			Namespace: "default",
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: ptr.To(int32(60 * 60)), // 1 hour
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "prefect-server"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "prefect-server-migration",
							Image:   "prefecthq/prefect:2.10.4",
							Command: []string{"prefect", "server", "database", "upgrade", "--yes"},
							Env: []corev1.EnvVar{
								{Name: "PREFECT_SERVER_DATABASE_CONNECTION_URL", Value: "postgresql://user:password@host:5432/db"},
							},
						},
					},
					RestartPolicy: corev1.RestartPolicyOnFailure,
				},
			},
		},
	}
}

var _ = Describe("Hash Function", func() {
	It("returns the same hash for the same job spec", func() {
		By("comparing hash for the same job spec")
		job := MigrationJobStub()
		job2 := MigrationJobStub()
		hash1, err := Hash(job.Spec, 8)
		Expect(err).NotTo(HaveOccurred())
		hash2, err := Hash(job2.Spec, 8)
		Expect(err).NotTo(HaveOccurred())

		Expect(hash1).To(Equal(hash2))
	})

	It("returns different hashes for different job specs", func() {
		By("comparing hash for different job specs")
		job := MigrationJobStub()
		hash1, err := Hash(job.Spec, 8)
		Expect(err).NotTo(HaveOccurred())

		job.Spec.Template.Spec.Containers[0].Image = "image1:v2"
		hash2, err := Hash(job.Spec, 8)
		Expect(err).NotTo(HaveOccurred())

		Expect(hash1).NotTo(Equal(hash2))
	})
})
