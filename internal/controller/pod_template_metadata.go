package controller

import (
	appsv1 "k8s.io/api/apps/v1"

	"github.com/PrefectHQ/prefect-operator/internal/podtemplate"
)

// setPodTemplateMetadata sets the pod template labels and annotations of
// deploy, whose pod template has just been replaced with the desired one. It
// keeps the ones other tools added to the live pod template, given as
// liveLabels and liveAnnotations, and records the ones the operator set on
// deploy.
func setPodTemplateMetadata(deploy *appsv1.Deployment, liveLabels, liveAnnotations map[string]string) {
	deploy.Spec.Template.Labels = mergeManaged(deploy, liveLabels, deploy.Spec.Template.Labels, podtemplate.ManagedLabelsAnnotation)
	deploy.Spec.Template.Annotations = mergeManaged(deploy, liveAnnotations, deploy.Spec.Template.Annotations, podtemplate.ManagedAnnotationsAnnotation)
}

func mergeManaged(deploy *appsv1.Deployment, live, desired map[string]string, record string) map[string]string {
	merged, managed := podtemplate.MergeManaged(live, desired, deploy.Annotations[record])

	if managed == "" {
		delete(deploy.Annotations, record)
	} else {
		if deploy.Annotations == nil {
			deploy.Annotations = map[string]string{}
		}
		deploy.Annotations[record] = managed
	}
	return merged
}
