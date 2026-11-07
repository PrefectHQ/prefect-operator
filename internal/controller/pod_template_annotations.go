package controller

import (
	appsv1 "k8s.io/api/apps/v1"

	"github.com/PrefectHQ/prefect-operator/internal/podtemplate"
)

// setPodTemplateAnnotations sets the pod template annotations of deploy, whose
// pod template has just been replaced with the desired one. It keeps the
// annotations other tools added to live, the pod template annotations before
// the replacement, and records the ones the operator set on deploy.
func setPodTemplateAnnotations(deploy *appsv1.Deployment, live map[string]string) {
	annotations, managed := podtemplate.MergeAnnotations(
		live,
		deploy.Spec.Template.Annotations,
		deploy.Annotations[podtemplate.ManagedAnnotationsAnnotation],
	)
	deploy.Spec.Template.Annotations = annotations

	if managed == "" {
		delete(deploy.Annotations, podtemplate.ManagedAnnotationsAnnotation)
		return
	}
	if deploy.Annotations == nil {
		deploy.Annotations = map[string]string{}
	}
	deploy.Annotations[podtemplate.ManagedAnnotationsAnnotation] = managed
}
