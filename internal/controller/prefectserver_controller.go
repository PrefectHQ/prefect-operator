package controller

import (
	"context"
	errs "errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"dario.cat/mergo"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
	"github.com/PrefectHQ/prefect-operator/internal/conditions"
	"github.com/PrefectHQ/prefect-operator/internal/constants"
	"github.com/PrefectHQ/prefect-operator/internal/podtemplate"
	"github.com/PrefectHQ/prefect-operator/internal/utils"
	"github.com/go-logr/logr"
)

// PrefectServerReconciler reconciles a PrefectServer object
type PrefectServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// Recorder emits Kubernetes events on the PrefectServers this controller
	// reconciles.
	Recorder events.EventRecorder
}

// The paths of the PrefectServer's pod templates, as reported in errors.
const (
	deploymentTemplateField   = "spec.deploymentTemplate"
	migrationJobTemplateField = "spec.migrationJobTemplate"
)

//+kubebuilder:rbac:groups=prefect.io,resources=prefectservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=prefect.io,resources=prefectservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=prefect.io,resources=prefectservers/finalizers,verbs=update
//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *PrefectServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrllog.FromContext(ctx)

	log.V(1).Info("Reconciling PrefectServer")

	server := &prefectiov1.PrefectServer{}
	err := r.Get(ctx, req.NamespacedName, server)
	if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// Defer a final status update at the end of the reconciliation loop, so that any of the
	// individual reconciliation functions can update the status as they see fit.
	defer func() {
		if statusErr := r.Status().Update(ctx, server); statusErr != nil {
			log.Error(statusErr, "Failed to update PrefectServer status")
		}
	}()

	// Surface (as a status condition, not an error) a multi-replica server
	// running on per-replica in-memory state — see checkSharedState.
	r.checkSharedState(server)

	desiredDeployment, desiredPVC, desiredMigrationJob, ignored, err := r.prefectServerDeployment(server)
	if invalid := (*podtemplate.InvalidTemplateError)(nil); errs.As(err, &invalid) {
		object := constants.Deployment
		if invalid.Field == migrationJobTemplateField {
			object = constants.MigrationJob
		}
		meta.SetStatusCondition(&server.Status.Conditions, conditions.InvalidPodTemplate(object, err))
		r.Recorder.Eventf(server, nil, corev1.EventTypeWarning, "InvalidPodTemplate", eventActionSync, "%v", err)
		// Retrying won't help until the user fixes the template, and that
		// change triggers a new reconcile.
		return ctrl.Result{}, reconcile.TerminalError(err)
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	reportPodTemplate(r.Recorder, server, &server.Status.Conditions, constants.Deployment, deploymentTemplateField, server.Spec.DeploymentTemplate, ignored.deployment)
	var migrationJobTemplate *runtime.RawExtension
	if desiredMigrationJob != nil {
		migrationJobTemplate = server.Spec.MigrationJobTemplate
	}
	reportPodTemplate(r.Recorder, server, &server.Status.Conditions, constants.MigrationJob, migrationJobTemplateField, migrationJobTemplate, ignored.migrationJob)

	desiredService, err := r.prefectServerService(server)
	if err != nil {
		return ctrl.Result{}, err
	}

	var result *ctrl.Result

	// Reconcile the PVC, if one is required
	result, err = r.reconcilePVC(ctx, server, desiredPVC, log)
	if result != nil {
		return *result, err
	}

	// Reconcile the migration job, if one is required.
	// NOTE: if an active migration is still running,
	// this reconciliation will requeue and exit early here.
	result, err = r.reconcileMigrationJob(ctx, server, desiredMigrationJob, log)
	if result != nil {
		return *result, err
	}

	// Reconcile the Deployment
	result, err = r.reconcileDeployment(ctx, server, desiredDeployment, log)
	if result != nil {
		return *result, err
	}

	// Reconcile the Service
	result, err = r.reconcileService(ctx, server, desiredService, log)
	if result != nil {
		return *result, err
	}

	return ctrl.Result{}, nil
}

// checkSharedState sets the SharedStateConfigured status condition. With
// replicas > 1, concurrency leases default to each replica's memory, so
// deployment/global concurrency limits under-enforce unless Redis-backed lease
// storage (spec.redis.leaseStorage) shares them. This is a warning condition
// rather than a validation error: single-replica servers and multi-replica
// servers that don't use concurrency limits work fine without it.
func (r *PrefectServerReconciler) checkSharedState(server *prefectiov1.PrefectServer) {
	replicas := server.Replicas()
	if replicas == nil || *replicas <= 1 {
		meta.SetStatusCondition(&server.Status.Conditions, conditions.SharedStateConfigured("SingleReplica"))
		return
	}

	redis := server.Spec.Redis
	if redis != nil && redis.LeaseStorage != nil && *redis.LeaseStorage {
		meta.SetStatusCondition(&server.Status.Conditions, conditions.SharedStateConfigured("RedisLeaseStorage"))
		return
	}

	meta.SetStatusCondition(&server.Status.Conditions, conditions.SharedStateInMemory())
}

func (r *PrefectServerReconciler) reconcilePVC(ctx context.Context, server *prefectiov1.PrefectServer, desiredPVC *corev1.PersistentVolumeClaim, log logr.Logger) (*ctrl.Result, error) {
	objName := constants.PVC

	if desiredPVC == nil {
		meta.SetStatusCondition(&server.Status.Conditions, conditions.NotRequired(objName))
		return nil, nil
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      desiredPVC.Name,
			Namespace: server.Namespace,
			Labels:    server.ServerLabels(),
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, pvc, func() error {
		if err := ctrl.SetControllerReference(server, pvc, r.Scheme); err != nil {
			return err
		}

		return mergo.Merge(pvc, desiredPVC, mergo.WithOverride)
	})

	log.V(1).Info("CreateOrUpdate", "object", objName, "name", server.Name, "result", result)

	meta.SetStatusCondition(
		&server.Status.Conditions,
		conditions.GetStatusConditionForOperationResult(result, objName, err),
	)

	if err != nil {
		return &ctrl.Result{}, err
	}

	return nil, nil
}

// migrationJobPollInterval is how often the reconciler checks on a migration
// Job. It doesn't watch Jobs, so it requeues until the Job is done.
const migrationJobPollInterval = 20 * time.Second

// reconcileMigrationJob creates the server's migration Job and holds the
// server's rollout until the Job completes, so a new server never starts on a
// database that has not been migrated for it.
func (r *PrefectServerReconciler) reconcileMigrationJob(ctx context.Context, server *prefectiov1.PrefectServer, desiredMigrationJob *batchv1.Job, log logr.Logger) (*ctrl.Result, error) {
	objName := constants.MigrationJob

	if desiredMigrationJob == nil {
		meta.SetStatusCondition(&server.Status.Conditions, conditions.NotRequired(objName))
		return nil, nil
	}

	foundMigrationJob := &batchv1.Job{}
	err := r.Get(ctx, types.NamespacedName{Namespace: server.Namespace, Name: desiredMigrationJob.Name}, foundMigrationJob)

	switch {
	case errors.IsNotFound(err):
		log.Info("Creating migration Job", "name", desiredMigrationJob.Name)
		if err = r.Create(ctx, desiredMigrationJob); err != nil {
			meta.SetStatusCondition(&server.Status.Conditions, conditions.NotCreated(objName, err))
			return &ctrl.Result{}, err
		}
		meta.SetStatusCondition(&server.Status.Conditions, conditions.Created(objName))
		return &ctrl.Result{RequeueAfter: migrationJobPollInterval}, nil

	case err != nil:
		meta.SetStatusCondition(&server.Status.Conditions, conditions.UnknownError(objName, err))
		return &ctrl.Result{}, err
	}

	if failed := jobCondition(foundMigrationJob, batchv1.JobFailed); failed != nil {
		// Kubernetes deletes the failed Job when its TTL runs out, and the
		// next reconcile creates it again.
		log.Info("Migration Job failed, not rolling out the server", "name", foundMigrationJob.Name, "reason", failed.Reason)
		meta.SetStatusCondition(
			&server.Status.Conditions,
			conditions.Failed(objName, fmt.Sprintf("migration Job %s failed: %s", foundMigrationJob.Name, failed.Message)),
		)
		return &ctrl.Result{RequeueAfter: migrationJobPollInterval}, nil
	}

	if jobCondition(foundMigrationJob, batchv1.JobComplete) == nil {
		log.Info("Waiting on active migration Job to complete", "name", foundMigrationJob.Name)
		meta.SetStatusCondition(
			&server.Status.Conditions,
			conditions.AlreadyExists(
				objName,
				fmt.Sprintf("migration Job %s is still active", foundMigrationJob.Name),
			),
		)
		return &ctrl.Result{RequeueAfter: migrationJobPollInterval}, nil
	}

	if !meta.IsStatusConditionTrue(server.Status.Conditions, "MigrationJobReconciled") {
		meta.SetStatusCondition(&server.Status.Conditions, conditions.Updated(objName))
	}
	return nil, nil
}

func (r *PrefectServerReconciler) reconcileDeployment(ctx context.Context, server *prefectiov1.PrefectServer, desiredDeployment appsv1.Deployment, log logr.Logger) (*ctrl.Result, error) {
	objName := constants.Deployment

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name,
			Namespace: server.Namespace,
			Labels:    server.ServerLabels(),
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, deploy, func() error {
		if err := ctrl.SetControllerReference(server, deploy, r.Scheme); err != nil {
			return err
		}

		liveLabels := maps.Clone(deploy.Spec.Template.Labels)
		liveAnnotations := maps.Clone(deploy.Spec.Template.Annotations)

		if err := mergo.Merge(deploy, desiredDeployment, mergo.WithOverride); err != nil {
			return err
		}

		// mergo skips empty values and merges maps, so assign the pod template
		// directly to ensure that anything removed from the server, or from its
		// deploymentTemplate, is also removed here.
		deploy.Spec.Template = *desiredDeployment.Spec.Template.DeepCopy()
		setPodTemplateMetadata(deploy, liveLabels, liveAnnotations)

		return nil
	})

	log.V(1).Info("CreateOrUpdate", "object", objName, "name", server.Name, "result", result)

	meta.SetStatusCondition(
		&server.Status.Conditions,
		conditions.GetStatusConditionForOperationResult(result, objName, err),
	)

	if err != nil {
		return &ctrl.Result{}, err
	}

	serverContainer := podtemplate.FindContainer(&desiredDeployment.Spec.Template.Spec, prefectiov1.ContainerNamePrefectServer)

	switch result {
	case controllerutil.OperationResultCreated:
		server.Status.Version = prefectiov1.VersionFromImage(serverContainer.Image)
	case controllerutil.OperationResultUpdated:
		ready := deploy.Status.ReadyReplicas > 0
		version := prefectiov1.VersionFromImage(serverContainer.Image)
		if server.Status.Ready != ready || server.Status.Version != version {
			server.Status.Ready = ready
			server.Status.Version = version
		}
	}

	return nil, nil
}

func (r *PrefectServerReconciler) reconcileService(ctx context.Context, server *prefectiov1.PrefectServer, desiredService corev1.Service, log logr.Logger) (*ctrl.Result, error) {
	objName := constants.Service

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name,
			Namespace: server.Namespace,
			Labels:    server.ServiceLabels(),
		},
	}

	result, err := controllerutil.CreateOrUpdate(ctx, r.Client, service, func() error {
		if err := ctrl.SetControllerReference(server, service, r.Scheme); err != nil {
			return err
		}

		return mergo.Merge(service, desiredService, mergo.WithOverride)
	})

	log.V(1).Info("CreateOrUpdate", "object", objName, "name", server.Name, "result", result)

	meta.SetStatusCondition(
		&server.Status.Conditions,
		conditions.GetStatusConditionForOperationResult(result, objName, err),
	)

	if err != nil {
		return &ctrl.Result{}, err
	}

	return nil, nil
}

// jobCondition returns the Job's condition of the given type if it is true,
// or nil. The Job controller sets Complete or Failed only once the Job is
// done. A failed pod alone does not fail the Job, which retries the pod
// until it reaches its backoff limit.
func jobCondition(job *batchv1.Job, conditionType batchv1.JobConditionType) *batchv1.JobCondition {
	for i := range job.Status.Conditions {
		if c := &job.Status.Conditions[i]; c.Type == conditionType && c.Status == corev1.ConditionTrue {
			return c
		}
	}
	return nil
}

// ignoredFields lists the protected fields that the server's pod templates
// tried to override.
type ignoredFields struct {
	deployment   []string
	migrationJob []string
}

func (r *PrefectServerReconciler) prefectServerDeployment(server *prefectiov1.PrefectServer) (appsv1.Deployment, *corev1.PersistentVolumeClaim, *batchv1.Job, ignoredFields, error) {
	var pvc *corev1.PersistentVolumeClaim
	var migrationJob *batchv1.Job
	var deploymentSpec appsv1.DeploymentSpec
	var ignored ignoredFields

	if server.Spec.SQLite != nil {
		pvc = r.sqlitePersistentVolumeClaim(server)
		deploymentSpec = r.sqliteDeploymentSpec(server, pvc)
	} else if server.Spec.Postgres != nil {
		var err error
		if migrationJob, ignored.migrationJob, err = r.postgresMigrationJob(server); err != nil {
			return appsv1.Deployment{}, nil, nil, ignoredFields{}, err
		}
		deploymentSpec = r.postgresDeploymentSpec(server)
		deploymentSpec.Replicas = server.Replicas()
	} else {
		if server.Spec.Ephemeral == nil {
			server.Spec.Ephemeral = &prefectiov1.EphemeralConfiguration{}
		}
		deploymentSpec = r.ephemeralDeploymentSpec(server)
	}

	deploymentSpec.Template.Spec.Affinity = server.Spec.Affinity
	deploymentSpec.Template.Spec.Tolerations = server.Spec.Tolerations

	// Append any extra containers into the Deployment if configured.
	if server.Spec.ExtraContainers != nil {
		deploymentSpec.Template.Spec.Containers = append(deploymentSpec.Template.Spec.Containers, server.Spec.ExtraContainers...)
	}

	template, deploymentIgnored, err := podtemplate.Apply(deploymentSpec.Template, server.Spec.DeploymentTemplate, deploymentTemplateField, serverProtected(server, server.ServerLabels()))
	if err != nil {
		return appsv1.Deployment{}, nil, nil, ignoredFields{}, err
	}
	ignored.deployment = deploymentIgnored
	deploymentSpec.Template = template

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name,
			Namespace: server.Namespace,
			Labels:    server.ServerLabels(),
		},
		Spec: deploymentSpec,
	}

	// Set PrefectServer instance as the owner and controller
	if r.Scheme == nil {
		return appsv1.Deployment{}, nil, nil, ignoredFields{}, fmt.Errorf("scheme is nil, cannot set controller reference for deployment")
	}
	if err := ctrl.SetControllerReference(server, dep, r.Scheme); err != nil {
		return appsv1.Deployment{}, nil, nil, ignoredFields{}, fmt.Errorf("failed to set controller reference for deployment: %w", err)
	}
	if pvc != nil {
		if err := ctrl.SetControllerReference(server, pvc, r.Scheme); err != nil {
			return appsv1.Deployment{}, nil, nil, ignoredFields{}, fmt.Errorf("failed to set controller reference for PVC: %w", err)
		}
	}
	if migrationJob != nil {
		if err := ctrl.SetControllerReference(server, migrationJob, r.Scheme); err != nil {
			return appsv1.Deployment{}, nil, nil, ignoredFields{}, fmt.Errorf("failed to set controller reference for migration job: %w", err)
		}
	}
	return *dep, pvc, migrationJob, ignored, nil
}

func (r *PrefectServerReconciler) ephemeralDeploymentSpec(server *prefectiov1.PrefectServer) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
		Replicas: new(int32(1)),
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: server.ServerLabels(),
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: server.ServerLabels(),
			},
			Spec: corev1.PodSpec{
				NodeSelector: server.Spec.NodeSelector,
				Volumes: []corev1.Volume{
					{
						Name: constants.PrefectDataVolumeName,
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name: prefectiov1.ContainerNamePrefectServer,

						Image:           server.Image(),
						ImagePullPolicy: corev1.PullIfNotPresent,

						Args: server.EntrypointArguments(),
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      constants.PrefectDataVolumeName,
								MountPath: constants.PrefectHomeMountPath,
							},
						},
						Env: server.ToEnvVars(),
						Ports: []corev1.ContainerPort{
							{
								Name:          constants.APIPortName,
								ContainerPort: 4200,
								Protocol:      corev1.ProtocolTCP,
							},
						},

						Resources: server.Spec.Resources,

						StartupProbe:   server.StartupProbe(),
						ReadinessProbe: server.ReadinessProbe(),
						LivenessProbe:  server.LivenessProbe(),

						TerminationMessagePath:   constants.TerminationMessagePath,
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
					},
				},
			},
		},
	}
}

func (r *PrefectServerReconciler) sqlitePersistentVolumeClaim(server *prefectiov1.PrefectServer) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: server.Namespace,
			Name:      server.Name + "-data",
			Labels:    server.ServerLabels(),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &server.Spec.SQLite.StorageClassName,
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: server.Spec.SQLite.Size,
				},
			},
		},
	}
}

func (r *PrefectServerReconciler) sqliteDeploymentSpec(server *prefectiov1.PrefectServer, pvc *corev1.PersistentVolumeClaim) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
		Replicas: new(int32(1)),
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RecreateDeploymentStrategyType,
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: server.ServerLabels(),
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: server.ServerLabels(),
			},
			Spec: corev1.PodSpec{
				NodeSelector: server.Spec.NodeSelector,
				Volumes: []corev1.Volume{
					{
						Name: constants.PrefectDataVolumeName,
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: pvc.Name,
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name: prefectiov1.ContainerNamePrefectServer,

						Image:           server.Image(),
						ImagePullPolicy: corev1.PullIfNotPresent,

						Args: server.EntrypointArguments(),
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      constants.PrefectDataVolumeName,
								MountPath: constants.PrefectHomeMountPath,
							},
						},
						Env: server.ToEnvVars(),
						Ports: []corev1.ContainerPort{
							{
								Name:          constants.APIPortName,
								ContainerPort: 4200,
								Protocol:      corev1.ProtocolTCP,
							},
						},

						Resources: server.Spec.Resources,

						StartupProbe:   server.StartupProbe(),
						ReadinessProbe: server.ReadinessProbe(),
						LivenessProbe:  server.LivenessProbe(),

						TerminationMessagePath:   constants.TerminationMessagePath,
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
					},
				},
			},
		},
	}
}

func (r *PrefectServerReconciler) postgresDeploymentSpec(server *prefectiov1.PrefectServer) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
		Replicas: new(int32(1)),
		Strategy: appsv1.DeploymentStrategy{
			Type: appsv1.RollingUpdateDeploymentStrategyType,
		},
		Selector: &metav1.LabelSelector{
			MatchLabels: server.ServerLabels(),
		},
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: server.ServerLabels(),
			},
			Spec: corev1.PodSpec{
				NodeSelector: server.Spec.NodeSelector,
				InitContainers: []corev1.Container{
					r.initContainerWaitForPostgres(server),
				},
				Containers: []corev1.Container{
					{
						Name: prefectiov1.ContainerNamePrefectServer,

						Image:           server.Image(),
						ImagePullPolicy: corev1.PullIfNotPresent,

						Args: server.EntrypointArguments(),
						Env:  server.ToEnvVars(),
						Ports: []corev1.ContainerPort{
							{
								Name:          constants.APIPortName,
								ContainerPort: 4200,
								Protocol:      corev1.ProtocolTCP,
							},
						},

						Resources: server.Spec.Resources,

						StartupProbe:   server.StartupProbe(),
						ReadinessProbe: server.ReadinessProbe(),
						LivenessProbe:  server.LivenessProbe(),

						TerminationMessagePath:   constants.TerminationMessagePath,
						TerminationMessagePolicy: corev1.TerminationMessageReadFile,
					},
				},
			},
		},
	}
}

// serverProtected returns the parts of the server's pod templates that the
// operator depends on, which a user's pod template can't override. labels are
// the pod template labels of the Deployment or the Job.
func serverProtected(server *prefectiov1.PrefectServer, labels map[string]string) podtemplate.Protected {
	derivedEnv := server.DerivedEnvVarNames()
	protected := podtemplate.Protected{
		Labels:  slices.Collect(maps.Keys(labels)),
		Volumes: []string{constants.PrefectDataVolumeName},
		Containers: map[string]podtemplate.ProtectedContainer{
			prefectiov1.ContainerNamePrefectServer: {
				Ports:        []string{constants.APIPortName},
				VolumeMounts: []string{constants.PrefectDataVolumeName},
				Env:          derivedEnv,
			},
			prefectiov1.ContainerNameMigration: {
				Env: derivedEnv,
			},
		},
	}
	if server.Spec.Postgres != nil {
		protected.Containers[prefectiov1.ContainerNameWaitForDatabase] = podtemplate.ProtectedContainer{
			Env: []string{server.Spec.Postgres.HostEnvVar().Name, server.Spec.Postgres.UserEnvVar().Name},
		}
	}
	return protected
}

// migrationJobAffinity returns the node affinity of the server, if any. Pod affinity terms are
// left out because they usually target the server's own pods, which aren't created or updated
// until the migration Job finishes, so requiring them could leave the Job unschedulable.
func migrationJobAffinity(server *prefectiov1.PrefectServer) *corev1.Affinity {
	if server.Spec.Affinity == nil || server.Spec.Affinity.NodeAffinity == nil {
		return nil
	}
	return &corev1.Affinity{NodeAffinity: server.Spec.Affinity.NodeAffinity}
}

// postgresMigrationJob builds the migration Job. It also returns the protected
// fields its migrationJobTemplate tried to override.
func (r *PrefectServerReconciler) postgresMigrationJob(server *prefectiov1.PrefectServer) (*batchv1.Job, []string, error) {
	jobSpec := batchv1.JobSpec{
		TTLSecondsAfterFinished: new(int32(60 * 60)), // 1 hour
		Template: corev1.PodTemplateSpec{
			ObjectMeta: metav1.ObjectMeta{
				Labels: server.MigrationJobLabels(),
			},
			Spec: corev1.PodSpec{
				NodeSelector: server.Spec.NodeSelector,
				Tolerations:  server.Spec.Tolerations,
				Affinity:     migrationJobAffinity(server),
				InitContainers: []corev1.Container{
					r.initContainerWaitForPostgres(server),
				},
				Containers: []corev1.Container{
					{
						Name:    prefectiov1.ContainerNameMigration,
						Image:   server.Image(),
						Command: []string{prefectiov1.PrefectCLI, prefectiov1.ServerSubcommand, "database", "upgrade", "--yes"},
						Env:     server.ToEnvVars(),
					},
				},
				RestartPolicy: corev1.RestartPolicyOnFailure,
			},
		},
	}

	template, ignored, err := podtemplate.Apply(jobSpec.Template, server.Spec.MigrationJobTemplate, migrationJobTemplateField, serverProtected(server, server.MigrationJobLabels()))
	if err != nil {
		return nil, nil, err
	}
	jobSpec.Template = template

	// Generate hash based on the job spec
	hashSuffix, _ := utils.Hash(jobSpec, 8) // Use the first 8 characters of the hash
	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: server.Namespace,
			Name:      fmt.Sprintf("%s-migration-%s", server.Name, hashSuffix),
			Labels:    server.MigrationJobLabels(),
		},
		Spec: jobSpec,
	}, ignored, nil
}

func (r *PrefectServerReconciler) prefectServerService(server *prefectiov1.PrefectServer) (corev1.Service, error) {
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: server.Namespace,
			Name:      server.Name,
			Labels:    server.ServiceLabels(),
		},
		Spec: corev1.ServiceSpec{
			Selector: server.ServerLabels(),
			Ports: []corev1.ServicePort{
				{
					Name:       constants.APIPortName,
					Protocol:   corev1.ProtocolTCP,
					Port:       4200,
					TargetPort: intstr.FromString(constants.APIPortName),
				},
			},
		},
	}
	// Append any extra ports into the Service if configured.
	if server.Spec.ExtraServicePorts != nil {
		service.Spec.Ports = append(service.Spec.Ports, server.Spec.ExtraServicePorts...)
	}

	if r.Scheme == nil {
		return corev1.Service{}, fmt.Errorf("scheme is nil, cannot set controller reference for service")
	}
	if err := ctrl.SetControllerReference(server, &service, r.Scheme); err != nil {
		return corev1.Service{}, fmt.Errorf("failed to set controller reference for service: %w", err)
	}

	return service, nil
}

func (r *PrefectServerReconciler) initContainerWaitForPostgres(server *prefectiov1.PrefectServer) corev1.Container {
	image := server.Spec.Postgres.WaitForDatabaseImage
	if image == "" {
		image = constants.DefaultPostgresWaitImage
	}

	return corev1.Container{
		Name:            prefectiov1.ContainerNameWaitForDatabase,
		Image:           image,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command: []string{
			"/bin/sh",
			"-c",
			"until pg_isready -h $PREFECT_API_DATABASE_HOST -U $PREFECT_API_DATABASE_USER; do echo 'Waiting for PostgreSQL...'; sleep 2; done;",
		},
		Env: []corev1.EnvVar{
			server.Spec.Postgres.HostEnvVar(),
			server.Spec.Postgres.UserEnvVar(),
		},
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *PrefectServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&prefectiov1.PrefectServer{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}
