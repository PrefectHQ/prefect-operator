package controller

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	conditionPVCNotRequired = metav1.Condition{
		Type:    "PersistentVolumeClaimReconciled",
		Status:  metav1.ConditionTrue,
		Reason:  "PersistentVolumeClaimNotRequired",
		Message: "PersistentVolumeClaim is not required",
	}

	conditionPVCCreated = metav1.Condition{
		Type:    "PersistentVolumeClaimReconciled",
		Status:  metav1.ConditionTrue,
		Reason:  "PersistentVolumeClaimCreated",
		Message: "PersistentVolumeClaim was created",
	}

	conditionPVCUpdated = metav1.Condition{
		Type:    "PersistentVolumeClaimReconciled",
		Status:  metav1.ConditionTrue,
		Reason:  "PersistentVolumeClaimUpdated",
		Message: "PersistentVolumeClaim is in the correct state",
	}

	conditionMigrationJobNotRequired = metav1.Condition{
		Type:    "MigrationJobReconciled",
		Status:  metav1.ConditionTrue,
		Reason:  "MigrationJobNotRequired",
		Message: "MigrationJob is not required",
	}
)

func notCreated(object string, err error) metav1.Condition {
	return metav1.Condition{
		Type:    fmt.Sprintf("%sReconciled", object),
		Status:  metav1.ConditionFalse,
		Reason:  fmt.Sprintf("%sNotCreated", object),
		Message: fmt.Sprintf("%s was not created: %v", object, err.Error()),
	}
}

func created(object string) metav1.Condition {
	return metav1.Condition{
		Type:    fmt.Sprintf("%sReconciled", object),
		Status:  metav1.ConditionTrue,
		Reason:  fmt.Sprintf("%sCreated", object),
		Message: fmt.Sprintf("%s was created", object),
	}
}

func conditionPVCNotCreated(err error) metav1.Condition {
	return metav1.Condition{
		Type:    "PersistentVolumeClaimReconciled",
		Status:  metav1.ConditionFalse,
		Reason:  "PersistentVolumeClaimNotCreated",
		Message: "PersistentVolumeClaim was not created: " + err.Error(),
	}
}

func conditionPVCUnknownError(err error) metav1.Condition {
	return metav1.Condition{
		Type:    "PersistentVolumeClaimReconciled",
		Status:  metav1.ConditionFalse,
		Reason:  "UnknownError",
		Message: "Unknown error: " + err.Error(),
	}
}

func conditionUnknownError(object string, err error) metav1.Condition {
	return metav1.Condition{
		Type:    fmt.Sprintf("%sReconciled", object),
		Status:  metav1.ConditionFalse,
		Reason:  "UnknownError",
		Message: "Unknown error: " + err.Error(),
	}
}

func conditionPVCAlreadyExists(message string) metav1.Condition {
	return metav1.Condition{
		Type:    "PersistentVolumeClaimReconciled",
		Status:  metav1.ConditionFalse,
		Reason:  "PersistentVolumeClaimAlreadyExists",
		Message: message,
	}
}

func conditionAlreadyExists(object string, message string) metav1.Condition {
	return metav1.Condition{
		Type:    fmt.Sprintf("%sReconciled", object),
		Status:  metav1.ConditionFalse,
		Reason:  fmt.Sprintf("%sAlreadyExists", object),
		Message: message,
	}
}
func conditionUpdated(object string) metav1.Condition {
	return metav1.Condition{
		Type:    fmt.Sprintf("%sReconciled", object),
		Status:  metav1.ConditionTrue,
		Reason:  fmt.Sprintf("%sUpdated", object),
		Message: fmt.Sprintf("%s is in the correct state", object),
	}
}

func conditionNeedsUpdate(object string) metav1.Condition {
	return metav1.Condition{
		Type:    fmt.Sprintf("%sReconciled", object),
		Status:  metav1.ConditionFalse,
		Reason:  fmt.Sprintf("%sNeedsUpdate", object),
		Message: fmt.Sprintf("%s needs to be updated", object),
	}
}

func conditionUpdateFailed(object string, err error) metav1.Condition {
	return metav1.Condition{
		Type:    fmt.Sprintf("%sReconciled", object),
		Status:  metav1.ConditionFalse,
		Reason:  fmt.Sprintf("%sUpdateFailed", object),
		Message: fmt.Sprintf("%s update failed: %v", object, err.Error()),
	}
}
