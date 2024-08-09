/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	prefectiov1 "github.com/PrefectHQ/prefect-operator/api/v1"
)

// PrefectServerReconciler reconciles a PrefectServer object
type PrefectServerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=prefect.io,resources=prefectservers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=prefect.io,resources=prefectservers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=prefect.io,resources=prefectservers/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=events,verbs=create;patch
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// TODO(user): Modify the Reconcile function to compare the state specified by
// the PrefectServer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.3/pkg/reconcile
func (r *PrefectServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	server := &prefectiov1.PrefectServer{}
	err := r.Get(ctx, req.NamespacedName, server)
	if errors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	desiredDeployment, desiredPVC := r.prefectServerDeployment(server)
	desiredService := r.prefectServerService(server)

	// Reconcile the PVC, if one is required
	if desiredPVC != nil {
		foundPVC := &corev1.PersistentVolumeClaim{}
		err = r.Get(ctx, types.NamespacedName{Name: desiredPVC.Name, Namespace: server.Namespace}, foundPVC)
		if errors.IsNotFound(err) {
			if err = r.Create(ctx, desiredPVC); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		} else if err != nil {
			return ctrl.Result{}, err
		} else if !metav1.IsControlledBy(foundPVC, server) {
			return ctrl.Result{}, errors.NewBadRequest("PVC already exists and is not controlled by PrefectServer")
		} else {
			// TODO: handle patching the PVC if there are meaningful updates
		}
	}

	// Reconcile the Deployment
	foundDeployment := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Name: server.Name, Namespace: server.Namespace}, foundDeployment)
	if errors.IsNotFound(err) {
		if err = r.Create(ctx, &desiredDeployment); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	} else if !metav1.IsControlledBy(foundDeployment, server) {
		return ctrl.Result{}, errors.NewBadRequest("Deployment already exists and is not controlled by PrefectServer")
	} else {
		if err = r.Update(ctx, &desiredDeployment); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Reconcile the Service
	foundService := &corev1.Service{}
	err = r.Get(ctx, types.NamespacedName{Name: server.Name, Namespace: server.Namespace}, foundService)
	if errors.IsNotFound(err) {
		if err = r.Create(ctx, &desiredService); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	} else if !metav1.IsControlledBy(foundService, server) {
		return ctrl.Result{}, errors.NewBadRequest("Service already exists and is not controlled by PrefectServer")
	} else {
		if err = r.Update(ctx, &desiredService); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *PrefectServerReconciler) prefectServerDeployment(server *prefectiov1.PrefectServer) (appsv1.Deployment, *corev1.PersistentVolumeClaim) {
	var pvc *corev1.PersistentVolumeClaim
	var deploymentSpec appsv1.DeploymentSpec

	if server.Spec.SQLite != nil {
		pvc = &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: server.Namespace,
				Name:      server.Name + "-data",
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

		deploymentSpec = r.sqliteDeploymentSpec(server, pvc)
	} else if server.Spec.Postgres != nil {
		deploymentSpec = r.postgresDeploymentSpec(server)
	} else {
		if server.Spec.Ephemeral == nil {
			server.Spec.Ephemeral = &prefectiov1.EphemeralConfiguration{}
		}
		deploymentSpec = r.ephemeralDeploymentSpec(server)
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name,
			Namespace: server.Namespace,
		},
		Spec: deploymentSpec,
	}

	// Set PrefectServer instance as the owner and controller
	ctrl.SetControllerReference(server, dep, r.Scheme)
	if pvc != nil {
		ctrl.SetControllerReference(server, pvc, r.Scheme)
	}
	return *dep, pvc
}

func (r *PrefectServerReconciler) ephemeralDeploymentSpec(server *prefectiov1.PrefectServer) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
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
				Volumes: []corev1.Volume{
					{
						Name: "prefect-data",
						VolumeSource: corev1.VolumeSource{
							EmptyDir: &corev1.EmptyDirVolumeSource{},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name:    server.Name,
						Image:   server.Image(),
						Command: server.Command(),
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "prefect-data",
								MountPath: "/var/lib/prefect/",
							},
						},
						Env: append(append([]corev1.EnvVar{
							{
								Name:  "PREFECT_HOME",
								Value: "/var/lib/prefect/",
							},
						}, server.Spec.Ephemeral.ToEnvVars()...), server.Spec.Settings...),
						Ports: []corev1.ContainerPort{
							{
								Name:          "api",
								ContainerPort: 4200,
							},
						},
						StartupProbe:   server.StartupProbe(),
						ReadinessProbe: server.ReadinessProbe(),
						LivenessProbe:  server.LivenessProbe(),
					},
				},
			},
		},
	}
}

func (r *PrefectServerReconciler) sqliteDeploymentSpec(server *prefectiov1.PrefectServer, pvc *corev1.PersistentVolumeClaim) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
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
				Volumes: []corev1.Volume{
					{
						Name: pvc.Name,
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: pvc.Name,
							},
						},
					},
				},
				Containers: []corev1.Container{
					{
						Name:    server.Name,
						Image:   server.Image(),
						Command: server.Command(),
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      pvc.Name,
								MountPath: "/var/lib/prefect/",
							},
						},
						Env: append(append([]corev1.EnvVar{
							{
								Name:  "PREFECT_HOME",
								Value: "/var/lib/prefect/",
							},
						}, server.Spec.SQLite.ToEnvVars()...), server.Spec.Settings...),
						Ports: []corev1.ContainerPort{
							{
								Name:          "api",
								ContainerPort: 4200,
							},
						},
						StartupProbe:   server.StartupProbe(),
						ReadinessProbe: server.ReadinessProbe(),
						LivenessProbe:  server.LivenessProbe(),
					},
				},
			},
		},
	}
}

func (r *PrefectServerReconciler) postgresDeploymentSpec(server *prefectiov1.PrefectServer) appsv1.DeploymentSpec {
	return appsv1.DeploymentSpec{
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
				Containers: []corev1.Container{
					{
						Name:    server.Name,
						Image:   server.Image(),
						Command: server.Command(),
						Env: append(append([]corev1.EnvVar{
							{
								Name:  "PREFECT_HOME",
								Value: "/var/lib/prefect/",
							},
						}, server.Spec.Postgres.ToEnvVars()...), server.Spec.Settings...),
						Ports: []corev1.ContainerPort{
							{
								Name:          "api",
								ContainerPort: 4200,
							},
						},
						StartupProbe:   server.StartupProbe(),
						ReadinessProbe: server.ReadinessProbe(),
						LivenessProbe:  server.LivenessProbe(),
					},
				},
			},
		},
	}
}

func (r *PrefectServerReconciler) prefectServerService(server *prefectiov1.PrefectServer) corev1.Service {
	service := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      server.Name,
			Namespace: server.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": server.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "api",
					Protocol:   corev1.ProtocolTCP,
					Port:       4200,
					TargetPort: intstr.FromString("api"),
				},
			},
		},
	}

	ctrl.SetControllerReference(server, &service, r.Scheme)

	return service
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
