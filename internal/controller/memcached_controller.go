/*
Copyright 2025.

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
	"reflect"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	cachev1alpha1 "memcached-operator/api/v1alpha1"
)

// MemcachedReconciler reconciles a Memcached object
type MemcachedReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=cache.shivamkumar.dev,resources=memcacheds,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cache.shivamkumar.dev,resources=memcacheds/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=cache.shivamkumar.dev,resources=memcacheds/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Memcached object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.15.0/pkg/reconcile
func (r *MemcachedReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// fetch the Memcached instance
	memcached := &cachev1alpha1.Memcached{}
	if err := r.Get(ctx, req.NamespacedName, memcached); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Memcached resource not found, ignoring since the object has been deleted")
			// exit as the object has been deleted
			return ctrl.Result{}, nil
		}

		logger.Error(err, "Failed to get Memcached resource")
		// requeue because failed to fetch instance
		return ctrl.Result{}, err
	}

	// fetch the Deployment instance
	deployment := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: memcached.Name, Namespace: memcached.Namespace}, deployment); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("Deployment not found, creating a new one")
			if err := r.createNewDeployment(ctx, logger, memcached); err != nil {
				return ctrl.Result{}, err
			}

			// deployment created successfully, return and requeue
			return ctrl.Result{Requeue: true}, nil
		}

		// error fetching the Deployment, requeue
		logger.Error(err, "Failed to get Deployment")
		return ctrl.Result{}, err
	}

	// ensure number of replicas in the Deployment is same as the spec of Memcached
	desiredReplicas := int32(memcached.Spec.Size)
	currentReplicas := *deployment.Spec.Replicas
	if currentReplicas != desiredReplicas {
		// update the Deployment
		deployment.Spec.Replicas = &desiredReplicas
		if err := r.Update(ctx, deployment); err != nil {
			logger.Error(err, "Failed to update replicas in Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name, "CurrentReplicas", currentReplicas, "DesiredReplicas", desiredReplicas)
			// requeue because of failed update
			return ctrl.Result{}, err
		}

		// deployment updated successfully
		// add a delay to ensure pods are ready before going to next steps
		return ctrl.Result{RequeueAfter: time.Second * 90}, nil
	}

	// fetch list of Pods in the Deployment
	pods := &corev1.PodList{}
	if err := r.List(ctx, pods, client.InNamespace(memcached.Namespace), client.MatchingLabels(deployment.Spec.Selector.MatchLabels)); err != nil {
		logger.Error(err, "Failed to list Pods for the Deployment", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
		// requeue because of failed to list
		return ctrl.Result{}, err
	}

	// get the names of the pods in the Deployment
	expectedPodNames := getNameFromPodList(pods)
	currentPodNames := memcached.Status.Nodes

	// compare the expected and current pod names
	if !reflect.DeepEqual(expectedPodNames, currentPodNames) {
		// update status with the expected pod names
		memcached.Status.Nodes = expectedPodNames
		if err := r.Status().Update(ctx, memcached); err != nil {
			logger.Error(err, "Failed to update pod names in Memcached status", "Memcached.Namespace", memcached.Namespace, "Memcached.Name", memcached.Name, "ExpectedPodNames", expectedPodNames, "CurrentPodNames", currentPodNames)
			// requeue because of failed update
			return ctrl.Result{}, err
		}
	}

	// everything looks good, exit reconcile loop
	return ctrl.Result{}, nil
}

func (r *MemcachedReconciler) createNewDeployment(ctx context.Context, logger logr.Logger, memcached *cachev1alpha1.Memcached) error {
	newDeployment := r.getNewDeployment(memcached)
	logger.Info("Creating a new Deployment for Memcached", "Deployment.Namespace", newDeployment.Namespace, "Deployment.Name", newDeployment.Name)

	if err := r.Create(ctx, newDeployment); err != nil {
		logger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", newDeployment.Namespace, "Deployment.Name", newDeployment.Name)
		return err
	}

	return nil
}

func (r *MemcachedReconciler) getNewDeployment(memcached *cachev1alpha1.Memcached) *appsv1.Deployment {
	labels := getLabelsForMemcached(memcached)
	replicas := int32(memcached.Spec.Size)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      memcached.Name,
			Namespace: memcached.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  "memcached",
						Image: "memcached:1.6.37-alpine",
						Ports: []corev1.ContainerPort{{
							Name:          "memcached",
							ContainerPort: 11211,
						}},
					}},
				},
			},
			Replicas: &replicas,
		},
	}

	// set the owner of the Deployment to the Memcached instance
	ctrl.SetControllerReference(memcached, deployment, r.Scheme)

	return deployment
}

func getLabelsForMemcached(memcached *cachev1alpha1.Memcached) map[string]string {
	return map[string]string{"app": "memcached", "memcached_cr": memcached.Name}
}

func getNameFromPodList(pods *corev1.PodList) []string {
	var names []string
	for _, pod := range pods.Items {
		names = append(names, pod.Name)
	}
	return names
}

// SetupWithManager sets up the controller with the Manager.
func (r *MemcachedReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&cachev1alpha1.Memcached{}).
		Complete(r)
}
