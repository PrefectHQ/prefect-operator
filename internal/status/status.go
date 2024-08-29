package status

import (
	"github.com/PrefectHQ/prefect-operator/internal/conditions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func GetStatusConditionForOperationResult(result controllerutil.OperationResult, objectName string, err error) metav1.Condition {
	switch result {
	case controllerutil.OperationResultCreated:
		return conditions.Created(objectName)
	case controllerutil.OperationResultUpdated:
		return conditions.Updated(objectName)
	default:
		return conditions.UnknownError(objectName, err)
	}

	// Other OperationResult values we can check in the future if needed:
	// - OperationResultUpdatedStatus
	// - OperationResultUpdatedStatusOnly
	// - OperationResultNone
}
