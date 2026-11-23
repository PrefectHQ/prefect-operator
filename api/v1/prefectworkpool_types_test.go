package v1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("PrefectWorkPool type", func() {
	It("can be deep copied", func() {
		original := &PrefectWorkPool{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: PrefectWorkPoolSpec{
				Version: new("0.0.1"),
				Image:   new("prefecthq/prefect:0.0.1"),
				Server: PrefectServerReference{
					Namespace: "default",
					Name:      "prefect",
				},
				Type:    WorkPoolTypeK8s,
				Workers: int32(2),
			}}

		copied := original.DeepCopy()

		Expect(copied).To(Equal(original))
		Expect(copied).NotTo(BeIdenticalTo(original))
	})

	Describe("EntrypointArguments", func() {
		It("should use exact work pool name for names starting with 'prefect'", func() {
			workPool := &PrefectWorkPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prefect-work-pool",
				},
				Spec: PrefectWorkPoolSpec{
					Type: "kubernetes",
				},
			}

			args := workPool.EntrypointArguments()
			Expect(args).To(ContainElement("prefect-work-pool"))
			Expect(args).To(Equal([]string{
				PrefectCLI, "worker", StartCommand,
				WorkerArgPool, "prefect-work-pool", WorkerArgType, "kubernetes",
				WorkerArgWithHealthcheck,
			}))
		})

		It("should use exact work pool name for names not starting with 'prefect'", func() {
			workPool := &PrefectWorkPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "my-work-pool",
				},
				Spec: PrefectWorkPoolSpec{
					Type: "process",
				},
			}

			args := workPool.EntrypointArguments()
			Expect(args).To(ContainElement("my-work-pool"))
			Expect(args).To(Equal([]string{
				PrefectCLI, "worker", StartCommand,
				WorkerArgPool, "my-work-pool", WorkerArgType, "process",
				WorkerArgWithHealthcheck,
			}))
		})

		It("ensures naming consistency between controller and worker", func() {
			workPool := &PrefectWorkPool{
				ObjectMeta: metav1.ObjectMeta{
					Name: "prefect-production",
				},
				Spec: PrefectWorkPoolSpec{
					Type: "kubernetes",
				},
			}

			controllerName := workPool.Name

			workerArgs := workPool.EntrypointArguments()
			var workerPoolName string
			for i, arg := range workerArgs {
				if arg == "--pool" && i+1 < len(workerArgs) {
					workerPoolName = workerArgs[i+1]
					break
				}
			}

			Expect(controllerName).To(Equal("prefect-production"))
			Expect(workerPoolName).To(Equal("prefect-production"))
			Expect(controllerName).To(Equal(workerPoolName))
		})

		It("should work consistently for all naming patterns", func() {
			testCases := []struct {
				name         string
				expectedPool string
			}{
				{"prefect", "prefect"},
				{"prefect-k8s", "prefect-k8s"},
				{"prefect123", "prefect123"},
				{"k8s-pool", "k8s-pool"},
				{"my-prefect-pool", "my-prefect-pool"},
				{"PREFECT-POOL", "PREFECT-POOL"},
				{"production-pool", "production-pool"},
				{"dev", "dev"},
			}

			for _, tc := range testCases {
				workPool := &PrefectWorkPool{
					ObjectMeta: metav1.ObjectMeta{
						Name: tc.name,
					},
					Spec: PrefectWorkPoolSpec{
						Type: "process",
					},
				}

				args := workPool.EntrypointArguments()
				Expect(args).To(ContainElement(tc.expectedPool),
					"Work pool %s should use pool name %s", tc.name, tc.expectedPool)
			}
		})
	})
})

var _ = Describe("PrefectWorkPool.DerivedEnvVarNames", func() {
	It("names the variables the operator sets on the worker", func() {
		s := &PrefectWorkPool{Spec: PrefectWorkPoolSpec{
			Settings: []corev1.EnvVar{{Name: "EXTRA", Value: "1"}},
		}}

		Expect(s.DerivedEnvVarNames()).To(Equal([]string{
			EnvPrefectHome,
			"PREFECT_API_URL",
			"PREFECT_WORKER_WEBSERVER_PORT",
		}))
	})

	It("leaves out variables the settings override", func() {
		s := &PrefectWorkPool{Spec: PrefectWorkPoolSpec{
			Settings: []corev1.EnvVar{{Name: "PREFECT_WORKER_WEBSERVER_PORT", Value: "9090"}},
		}}

		Expect(s.DerivedEnvVarNames()).To(Equal([]string{EnvPrefectHome, "PREFECT_API_URL"}))
	})

	It("includes the API key when one is set", func() {
		s := &PrefectWorkPool{Spec: PrefectWorkPoolSpec{
			Server: PrefectServerReference{APIKey: &APIKeySpec{Value: new("key")}},
		}}

		Expect(s.DerivedEnvVarNames()).To(ContainElement("PREFECT_API_KEY"))
	})

	It("reads the auth string from its Secret when one is set", func() {
		ref := corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "prefect-auth"}, Key: "auth-string"}
		s := &PrefectWorkPool{Spec: PrefectWorkPoolSpec{
			Server: PrefectServerReference{Name: "prefect", Auth: &ServerAuth{SecretKeyRef: ref}},
		}}

		Expect(s.ToEnvVars()).To(ContainElement(corev1.EnvVar{
			Name: "PREFECT_API_AUTH_STRING", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &ref},
		}))
		Expect(s.DerivedEnvVarNames()).To(ContainElement("PREFECT_API_AUTH_STRING"))
	})

	It("names the auth string's Secret for the Jobs of a Kubernetes worker only", func() {
		secretName := corev1.EnvVar{Name: "PREFECT_INTEGRATIONS_KUBERNETES_WORKER_API_AUTH_STRING_SECRET_NAME", Value: "prefect-auth"}
		secretKey := corev1.EnvVar{Name: "PREFECT_INTEGRATIONS_KUBERNETES_WORKER_API_AUTH_STRING_SECRET_KEY", Value: "auth-string"}
		s := &PrefectWorkPool{Spec: PrefectWorkPoolSpec{
			Type: "kubernetes",
			Server: PrefectServerReference{Name: "prefect", Auth: &ServerAuth{SecretKeyRef: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: "prefect-auth"}, Key: "auth-string",
			}}},
		}}
		Expect(s.ToEnvVars()).To(ContainElements(secretName, secretKey))

		s.Spec.Type = "process"
		Expect(s.ToEnvVars()).NotTo(Or(ContainElement(secretName), ContainElement(secretKey)))

		s.Spec.Type = "kubernetes"
		s.Spec.Server.Auth = nil
		Expect(s.DerivedEnvVarNames()).NotTo(ContainElement(secretName.Name))
	})
})

var _ = Describe("PrefectWorkPool.ValidateSettings", func() {
	It("accepts other settings", func() {
		s := &PrefectWorkPool{Spec: PrefectWorkPoolSpec{
			Settings: []corev1.EnvVar{{Name: "PREFECT_LOGGING_LEVEL", Value: "DEBUG"}},
		}}
		Expect(s.ValidateSettings()).To(Succeed())
	})

	It("refuses settings that Server sets", func() {
		for _, name := range []string{"PREFECT_API_URL", "PREFECT_API_KEY", "PREFECT_API_AUTH_STRING"} {
			s := &PrefectWorkPool{Spec: PrefectWorkPoolSpec{
				Settings: []corev1.EnvVar{{Name: name, Value: "x"}},
			}}
			Expect(s.ValidateSettings()).To(MatchError("settings must not set " + name + "; set it with spec.server"))
		}
	})
})
