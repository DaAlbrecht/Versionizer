/*
Copyright 2023.

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

package controllers

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	mepflixisschv1 "github.com/lilbitsquishy/versionizer-operator/api/v1"

	appsv1 "k8s.io/api/apps/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	corev1 "k8s.io/api/core/v1"
)

// VersionizerReconciler reconciles a Versionizer object
type VersionizerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=mepflix.iss.ch.my.domain,resources=versionizers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mepflix.iss.ch.my.domain,resources=versionizers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=mepflix.iss.ch.my.domain,resources=versionizers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Versionizer object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *VersionizerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	versionizer := &mepflixisschv1.Versionizer{}
	err := r.Get(ctx, req.NamespacedName, versionizer)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("versionizer resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get versionizer")
		return ctrl.Result{}, err
	}

	found := &appsv1.Deployment{}
	err = r.Get(ctx, req.NamespacedName, found)
	if err != nil && apierrors.IsNotFound(err) {
		// Define a new deployment
		dep, err := r.deploymentForVersionizer(versionizer)
		if err != nil {
			log.Error(err, "Failed to build deployment for versionizer")
			return ctrl.Result{}, err
		}
		log.Info("Creating a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)

		if err := r.Create(ctx, dep); err != nil {
			log.Error(err, "Failed to create new Deployment",
				"Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Minute}, nil

	} else if err != nil {
		log.Info("Failed to get Deployment")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VersionizerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&mepflixisschv1.Versionizer{}).
		Owns(&appsv1.Deployment{}).
		Complete(r)
}

func (r *VersionizerReconciler) deploymentForVersionizer(versionizer *mepflixisschv1.Versionizer) (*appsv1.Deployment, error) {
	ConfigurationPath := versionizer.Spec.ConfigurationPath
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      versionizer.Name,
			Namespace: versionizer.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": versionizer.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot: &[]bool{false}[0],
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Containers: []corev1.Container{{
						Image:           "busybox:latest",
						Name:            "versionizer",
						ImagePullPolicy: corev1.PullAlways,
						SecurityContext: &corev1.SecurityContext{
							RunAsNonRoot: &[]bool{false}[0],
							Capabilities: &corev1.Capabilities{
								Drop: []corev1.Capability{
									"ALL",
								},
							},
						},
						Ports: []corev1.ContainerPort{{
							ContainerPort: 8080,
							Name:          "Versionizer",
						}},
						Env: []corev1.EnvVar{
							{
								Name:  "CONFIGURATION_PATH",
								Value: ConfigurationPath,
							},
							{
								Name:  "USERNAME",
								Value: "admin",
							},
							{
								Name: "PASSWORD",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "versionizer-secret",
										},
										Key:      "password",
										Optional: &[]bool{true}[0],
									},
								},
							},
						},
					},
					},
				},
			},
		}}
	if err := ctrl.SetControllerReference(versionizer, dep, r.Scheme); err != nil {
		return nil, err
	}

	return dep, nil
}
