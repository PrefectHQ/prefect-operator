package constants

const (
	// Built-in objects.
	PVC        = "PersistentVolumeClaim"
	Service    = "Service"
	Deployment = "Deployment"

	// Specific objects.
	MigrationJob = "MigrationJob"

	// Pod data.
	PrefectDataVolumeName  = "prefect-data"
	PrefectHomeMountPath   = "/var/lib/prefect/"
	TerminationMessagePath = "/dev/termination-log"
	APIPortName            = "api"
)
