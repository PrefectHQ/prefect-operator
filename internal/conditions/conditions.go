package conditions

import (
	"fmt"
	"strings"

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

func Failed(object string, message string) metav1.Condition {
	return metav1.Condition{
		Type:    fmt.Sprintf("%sReconciled", object),
		Reason:  fmt.Sprintf("%sFailed", object),
		Message: message,
		Status:  metav1.ConditionFalse,
	}
}

// InvalidPodTemplate reports that a user's pod template for object couldn't be applied.
func InvalidPodTemplate(object string, err error) metav1.Condition {
	return metav1.Condition{
		Type:    fmt.Sprintf("%sReconciled", object),
		Reason:  "InvalidPodTemplate",
		Message: err.Error(),
		Status:  metav1.ConditionFalse,
	}
}

// UnsupportedVersion reports that object was not changed because the Prefect
// version of its image does not support something in the spec.
func UnsupportedVersion(object string, err error) metav1.Condition {
	return metav1.Condition{
		Type:    fmt.Sprintf("%sReconciled", object),
		Reason:  "UnsupportedVersion",
		Message: err.Error(),
		Status:  metav1.ConditionFalse,
	}
}

// InvalidAuth reports that object was not changed because the Secret in
// spec.auth has no valid auth string, or because the server's settings set
// another one.
func InvalidAuth(object string, err error) metav1.Condition {
	return metav1.Condition{
		Type:    fmt.Sprintf("%sReconciled", object),
		Reason:  "InvalidAuth",
		Message: err.Error(),
		Status:  metav1.ConditionFalse,
	}
}

// AuthSupported is the type of the condition that reports whether the Prefect
// version of a server with spec.auth requires the auth string. It is True or
// Unknown only when the server also has a valid auth string.
const AuthSupported = "AuthSupported"

// AuthVersionSupported reports that the Prefect version of image requires the
// auth string.
func AuthVersionSupported(image string) metav1.Condition {
	return metav1.Condition{
		Type:    AuthSupported,
		Reason:  "VersionSupported",
		Message: fmt.Sprintf("the Prefect version of the image %q supports spec.auth", image),
		Status:  metav1.ConditionTrue,
	}
}

// AuthVersionUnsupported reports that the Prefect version of the server's image
// does not require the auth string.
func AuthVersionUnsupported(err error) metav1.Condition {
	return metav1.Condition{
		Type:    AuthSupported,
		Reason:  "UnsupportedVersion",
		Message: err.Error(),
		Status:  metav1.ConditionFalse,
	}
}

// AuthVersionUnknown reports that the operator can't tell whether the server
// requires the auth string.
func AuthVersionUnknown(message string) metav1.Condition {
	return metav1.Condition{
		Type:    AuthSupported,
		Reason:  "VersionUnknown",
		Message: message,
		Status:  metav1.ConditionUnknown,
	}
}

// TemplateApplied reports that the user's pod template at field was applied to
// object. ignored lists the protected fields the template tried to override,
// which kept the operator's values.
func TemplateApplied(object, field string, ignored []string) metav1.Condition {
	if len(ignored) == 0 {
		return metav1.Condition{
			Type:    fmt.Sprintf("%sTemplateApplied", object),
			Reason:  "TemplateApplied",
			Message: fmt.Sprintf("%s was applied", field),
			Status:  metav1.ConditionTrue,
		}
	}
	return metav1.Condition{
		Type:    fmt.Sprintf("%sTemplateApplied", object),
		Reason:  "ProtectedFieldsIgnored",
		Message: fmt.Sprintf("%s was applied, except for protected fields the operator keeps its own values for: %s", field, strings.Join(ignored, ", ")),
		Status:  metav1.ConditionTrue,
	}
}

// TypeSettingsApplied is the condition that SettingsIgnored returns.
const TypeSettingsApplied = "SettingsApplied"

// SettingsIgnored reports work pool settings that the operator left out of the
// worker, because spec.server sets those variables. names are the settings'
// names as written.
func SettingsIgnored(names []string) metav1.Condition {
	return metav1.Condition{
		Type:   TypeSettingsApplied,
		Reason: "ServerSettingsIgnored",
		Message: fmt.Sprintf("settings were applied, except for %s, which spec.server sets; "+
			"remove them from settings", strings.Join(names, ", ")),
		Status: metav1.ConditionTrue,
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

// SharedStateConfigured reports that a multi-replica server has Redis-backed
// concurrency lease storage (or doesn't need it at a single replica).
func SharedStateConfigured(reason string) metav1.Condition {
	return metav1.Condition{
		Type:    "SharedStateConfigured",
		Reason:  reason,
		Message: "server state shared across replicas is configured correctly",
		Status:  metav1.ConditionTrue,
	}
}

// SharedStateInMemory warns that replicas > 1 is running on per-replica
// in-memory state: concurrency leases (and the messaging broker) are not
// shared, so concurrency limits under-enforce across replicas and
// proactive/composite automation triggers may misfire.
func SharedStateInMemory() metav1.Condition {
	return metav1.Condition{
		Type:    "SharedStateConfigured",
		Reason:  "InMemoryLeaseStorage",
		Message: "replicas > 1 without spec.redis.leaseStorage: concurrency leases are held in each replica's memory, so concurrency limits under-enforce; configure spec.redis with leaseStorage: true",
		Status:  metav1.ConditionFalse,
	}
}

// DeletionBlocked reports that a PrefectServer being deleted keeps its
// finalizer because other resources still use it. dependents names some or
// all of them, and total is how many there are.
func DeletionBlocked(dependents []string, total int) metav1.Condition {
	subject := fmt.Sprintf("%d resources use", total)
	if total == 1 {
		subject = "1 resource uses"
	}
	message := fmt.Sprintf("%s this server and must be deleted first: %s", subject, strings.Join(dependents, ", "))
	if more := total - len(dependents); more > 0 {
		message += fmt.Sprintf(", and %d more", more)
	}
	return metav1.Condition{
		Type:    "DeletionBlocked",
		Reason:  "DependentsExist",
		Message: message,
		Status:  metav1.ConditionTrue,
	}
}
