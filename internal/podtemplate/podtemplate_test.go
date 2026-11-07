package podtemplate

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

const field = "spec.deploymentTemplate"

func base() corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{"app": "prefect-server"},
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{{
				Name:         "prefect-data",
				VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}},
			}},
			Containers: []corev1.Container{{
				Name:  "prefect-server",
				Image: "prefecthq/prefect:3.0.0-python3.12",
				Ports: []corev1.ContainerPort{{Name: "api", ContainerPort: 4200, Protocol: corev1.ProtocolTCP}},
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "prefect-data",
					MountPath: "/var/lib/prefect/",
				}},
				Env: []corev1.EnvVar{
					{Name: "PREFECT_HOME", Value: "/var/lib/prefect/"},
					{Name: "DB", Value: "sqlite"},
					{Name: "DB", Value: "override"},
				},
				ReadinessProbe: &corev1.Probe{
					ProbeHandler: corev1.ProbeHandler{
						HTTPGet: &corev1.HTTPGetAction{Path: "/api/ready"},
					},
					PeriodSeconds: 10,
				},
			}},
		},
	}
}

func protected() Protected {
	return Protected{
		Labels:  []string{"app"},
		Volumes: []string{"prefect-data"},
		Containers: map[string]ProtectedContainer{
			"prefect-server": {
				Ports:        []string{"api"},
				VolumeMounts: []string{"prefect-data"},
				Env:          []string{"PREFECT_HOME", "DB"},
			},
		},
	}
}

func apply(template string) (corev1.PodTemplateSpec, error) {
	return Apply(base(), &runtime.RawExtension{Raw: []byte(template)}, field, protected())
}

func mustApply(template string) corev1.PodTemplateSpec {
	result, err := apply(template)
	Expect(err).NotTo(HaveOccurred())
	return result
}

func server(template corev1.PodTemplateSpec) *corev1.Container {
	container := FindContainer(&template.Spec, "prefect-server")
	Expect(container).NotTo(BeNil())
	return container
}

var _ = Describe("Apply", func() {
	It("returns the base unchanged for a nil template", func() {
		result, err := Apply(base(), nil, field, protected())
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(base()))
	})

	It("returns the base unchanged for an empty template", func() {
		result, err := Apply(base(), &runtime.RawExtension{}, field, protected())
		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(base()))
	})

	It("overrides one probe field without restating the container", func() {
		result := mustApply(`{"spec":{"containers":[{"name":"prefect-server","readinessProbe":{"initialDelaySeconds":30}}]}}`)

		probe := server(result).ReadinessProbe
		Expect(probe.InitialDelaySeconds).To(Equal(int32(30)))
		Expect(probe.PeriodSeconds).To(Equal(int32(10)))
		Expect(probe.HTTPGet.Path).To(Equal("/api/ready"))
		Expect(server(result).Image).To(Equal("prefecthq/prefect:3.0.0-python3.12"))
	})

	It("sets pod-level fields", func() {
		result := mustApply(`{"spec":{"securityContext":{"runAsNonRoot":true,"runAsUser":1001}}}`)

		Expect(result.Spec.SecurityContext).NotTo(BeNil())
		Expect(result.Spec.SecurityContext.RunAsUser).To(Equal(new(int64(1001))))
		Expect(result.Spec.SecurityContext.RunAsNonRoot).To(Equal(new(true)))
	})

	It("adds labels and annotations", func() {
		result := mustApply(`{"metadata":{"labels":{"team":"data"},"annotations":{"a":"b"}}}`)

		Expect(result.Labels).To(Equal(map[string]string{"app": "prefect-server", "team": "data"}))
		Expect(result.Annotations).To(Equal(map[string]string{"a": "b"}))
	})

	It("ignores an override of a protected label", func() {
		result := mustApply(`{"metadata":{"labels":{"app":"other"}}}`)

		Expect(result.Labels).To(HaveKeyWithValue("app", "prefect-server"))
	})

	It("ignores an override of the protected port", func() {
		result := mustApply(`{"spec":{"containers":[{"name":"prefect-server","ports":[{"name":"api","containerPort":8000}]}]}}`)

		Expect(server(result).Ports).To(Equal(base().Spec.Containers[0].Ports))
	})

	It("ignores a replaced protected volume", func() {
		result := mustApply(`{"spec":{"volumes":[{"name":"prefect-data","hostPath":{"path":"/tmp"},"emptyDir":null}]}}`)

		Expect(result.Spec.Volumes).To(Equal(base().Spec.Volumes))
	})

	It("doesn't leave two mounts at a protected path", func() {
		result := mustApply(`{"spec":{"containers":[{"name":"prefect-server","volumeMounts":[{"name":"other","mountPath":"/var/lib/prefect/"}]}]}}`)

		Expect(server(result).VolumeMounts).To(Equal(base().Spec.Containers[0].VolumeMounts))
	})

	It("ignores overrides of protected env vars but keeps other env", func() {
		result := mustApply(`{"spec":{"containers":[{"name":"prefect-server","env":[{"name":"PREFECT_HOME","value":"/tmp"},{"name":"EXTRA","value":"1"}]}]}}`)

		env := server(result).Env
		Expect(env).To(ContainElement(corev1.EnvVar{Name: "EXTRA", Value: "1"}))
		Expect(env).To(ContainElement(corev1.EnvVar{Name: "PREFECT_HOME", Value: "/var/lib/prefect/"}))
		Expect(env).NotTo(ContainElement(corev1.EnvVar{Name: "PREFECT_HOME", Value: "/tmp"}))

		var db []string
		for _, e := range env {
			if e.Name == "DB" {
				db = append(db, e.Value)
			}
		}
		Expect(db).To(Equal([]string{"sqlite", "override"}))
	})

	It("keeps the base's env order when the template touches env", func() {
		withSettings := base()
		withSettings.Spec.Containers[0].Env = []corev1.EnvVar{
			{Name: "PREFECT_HOME", Value: "/var/lib/prefect/"},
			{Name: "NAME", Value: "derived"},
			{Name: "PW", Value: "secret"},
			{Name: "NAME", Value: "$(PW)"},
		}

		result, err := Apply(withSettings, &runtime.RawExtension{
			Raw: []byte(`{"spec":{"containers":[{"name":"prefect-server","env":[{"name":"EXTRA","value":"1"}]}]}}`),
		}, field, Protected{})
		Expect(err).NotTo(HaveOccurred())

		Expect(server(result).Env).To(Equal(append(withSettings.Spec.Containers[0].Env, corev1.EnvVar{Name: "EXTRA", Value: "1"})))
	})

	It("puts back a deleted protected container", func() {
		result := mustApply(`{"spec":{"containers":[{"name":"prefect-server","$patch":"delete"}]}}`)

		Expect(*server(result)).To(Equal(base().Spec.Containers[0]))
	})

	It("supports $patch: delete on an unprotected container", func() {
		withSidecar := base()
		withSidecar.Spec.Containers = append(withSidecar.Spec.Containers, corev1.Container{Name: "sidecar", Image: "busybox"})

		result, err := Apply(withSidecar, &runtime.RawExtension{
			Raw: []byte(`{"spec":{"containers":[{"name":"sidecar","$patch":"delete"}]}}`),
		}, field, protected())
		Expect(err).NotTo(HaveOccurred())

		Expect(FindContainer(&result.Spec, "sidecar")).To(BeNil())
		Expect(result.Spec.Containers).To(HaveLen(1))
	})

	It("rejects unknown fields", func() {
		_, err := apply(`{"spec":{"securtyContext":{}}}`)

		var invalid *InvalidTemplateError
		Expect(errors.As(err, &invalid)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("securtyContext"))
	})

	It("rejects malformed JSON", func() {
		_, err := apply(`{"spec":`)

		var invalid *InvalidTemplateError
		Expect(errors.As(err, &invalid)).To(BeTrue())
		Expect(invalid.Field).To(Equal(field))
		Expect(err.Error()).To(HavePrefix("invalid " + field + ": "))
	})
})

var _ = Describe("FindContainer", func() {
	It("returns nil for a missing name", func() {
		spec := base().Spec
		Expect(FindContainer(&spec, "missing")).To(BeNil())
	})
})
