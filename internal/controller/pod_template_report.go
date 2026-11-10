package controller

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"

	"github.com/PrefectHQ/prefect-operator/internal/conditions"
)

// reportPodTemplate sets the <object>TemplateApplied condition of obj for
// template, its pod template at field, or removes the condition if template is
// nil or empty, as when it isn't used. ignored lists the protected fields the template tried to override. A
// Warning event is emitted only when the condition changes, so a template
// that keeps trying to override the same fields doesn't emit one on every
// reconcile.
func reportPodTemplate(recorder events.EventRecorder, obj runtime.Object, statusConditions *[]metav1.Condition, object, field string, template *runtime.RawExtension, ignored []string) {
	if template == nil || len(template.Raw) == 0 {
		meta.RemoveStatusCondition(statusConditions, fmt.Sprintf("%sTemplateApplied", object))
		return
	}

	condition := conditions.TemplateApplied(object, field, ignored)
	if meta.SetStatusCondition(statusConditions, condition) && len(ignored) > 0 {
		recorder.Eventf(obj, nil, corev1.EventTypeWarning, condition.Reason, eventActionSync,
			"%s tried to override protected fields, which kept the operator's values: %s", field, strings.Join(ignored, ", "))
	}
}
