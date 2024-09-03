package conditions

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func GetStatusConditionForOperationResult(result controllerutil.OperationResult, objectName string, err error) metav1.Condition {
	switch result {
	case controllerutil.OperationResultCreated:
		return Created(objectName)
	case controllerutil.OperationResultUpdated:
		return Updated(objectName)
	default:
		return UnknownError(objectName, err)
	}

	// Other OperationResult values we can check in the future if needed:
	// - OperationResultUpdatedStatus
	// - OperationResultUpdatedStatusOnly
	// - OperationResultNone
}

func NotRequired(object string) metav1.Condition {
	return metav1.Condition{
		Type:    fmt.Sprintf("%sReconciled", object),
		Reason:  fmt.Sprintf("%sNotRequired", object),
		Message: fmt.Sprintf("%s is not required", object),
		Status:  metav1.ConditionTrue,
	}
}

func NotCreated(object string, err error) metav1.Condition {
	return metav1.Condition{
		Type:    fmt.Sprintf("%sReconciled", object),
		Reason:  fmt.Sprintf("%sNotCreated", object),
		Message: fmt.Sprintf("%s was not created: %v", object, err.Error()),
		Status:  metav1.ConditionFalse,
	}
}

func Created(object string) metav1.Condition {
	return metav1.Condition{
		Type:    fmt.Sprintf("%sReconciled", object),
		Reason:  fmt.Sprintf("%sCreated", object),
		Message: fmt.Sprintf("%s was created", object),
		Status:  metav1.ConditionTrue,
	}
}

func UnknownError(object string, err error) metav1.Condition {
	return metav1.Condition{
		Type:    fmt.Sprintf("%sReconciled", object),
		Reason:  "UnknownError",
		Message: "Unknown error: " + err.Error(),
		Status:  metav1.ConditionFalse,
	}
}

func AlreadyExists(object string, message string) metav1.Condition {
	return metav1.Condition{
		Type:    fmt.Sprintf("%sReconciled", object),
		Reason:  fmt.Sprintf("%sAlreadyExists", object),
		Message: message,
		Status:  metav1.ConditionFalse,
	}
}
func Updated(object string) metav1.Condition {
	return metav1.Condition{
		Type:    fmt.Sprintf("%sReconciled", object),
		Reason:  fmt.Sprintf("%sUpdated", object),
		Message: fmt.Sprintf("%s is in the correct state", object),
		Status:  metav1.ConditionTrue,
	}
}
