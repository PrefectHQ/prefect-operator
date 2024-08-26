package conditions

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newCondition(typeVal, reason, message string, status metav1.ConditionStatus) metav1.Condition {
	return metav1.Condition{
		Type:    typeVal,
		Status:  status,
		Reason:  reason,
		Message: message,
	}
}

func NotRequired(object string) metav1.Condition {
	return newCondition(
		fmt.Sprintf("%sReconciled", object),
		fmt.Sprintf("%sNotRequired", object),
		fmt.Sprintf("%s is not required", object),
		metav1.ConditionTrue,
	)
}

func NotCreated(object string, err error) metav1.Condition {
	return newCondition(
		fmt.Sprintf("%sReconciled", object),
		fmt.Sprintf("%sNotCreated", object),
		fmt.Sprintf("%s was not created: %v", object, err.Error()),
		metav1.ConditionFalse,
	)
}

func Created(object string) metav1.Condition {
	return newCondition(
		fmt.Sprintf("%sReconciled", object),
		fmt.Sprintf("%sCreated", object),
		fmt.Sprintf("%s was created", object),
		metav1.ConditionTrue,
	)
}

func UnknownError(object string, err error) metav1.Condition {
	return newCondition(
		fmt.Sprintf("%sReconciled", object),
		"UnknownError",
		"Unknown error: "+err.Error(),
		metav1.ConditionFalse,
	)
}

func AlreadyExists(object string, message string) metav1.Condition {
	return newCondition(
		fmt.Sprintf("%sReconciled", object),
		fmt.Sprintf("%sAlreadyExists", object),
		message,
		metav1.ConditionFalse,
	)
}
func Updated(object string) metav1.Condition {
	return newCondition(
		fmt.Sprintf("%sReconciled", object),
		fmt.Sprintf("%sUpdated", object),
		fmt.Sprintf("%s is in the correct state", object),
		metav1.ConditionTrue,
	)
}

func NeedsUpdate(object string) metav1.Condition {
	return newCondition(
		fmt.Sprintf("%sReconciled", object),
		fmt.Sprintf("%sNeedsUpdate", object),
		fmt.Sprintf("%s needs to be updated", object),
		metav1.ConditionFalse,
	)
}

func UpdateFailed(object string, err error) metav1.Condition {
	return newCondition(
		fmt.Sprintf("%sReconciled", object),
		fmt.Sprintf("%sUpdateFailed", object),
		fmt.Sprintf("%s update failed: %v", object, err.Error()),
		metav1.ConditionFalse,
	)
}
