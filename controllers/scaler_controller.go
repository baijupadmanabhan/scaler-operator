/*
Copyright 2023 Baiju Padmanabhan.

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
	"fmt"
	"time"

	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	apiv1alpha1 "github.com/baijupadmanabhan/scaler-operator/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var logger = log.Log.WithName("controller_scaler")

// ScalerReconciler reconciles a Scaler object
type ScalerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=api.ubuntu.matrix.com,resources=scalers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=api.ubuntu.matrix.com,resources=scalers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=api.ubuntu.matrix.com,resources=scalers/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Scaler object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.13.0/pkg/reconcile
func (r *ScalerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	//_ = log.FromContext(ctx)

	log := logger.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)
	log.Info("Invoking reconcile")

	scaler := &apiv1alpha1.Scaler{}

	err := r.Get(ctx, req.NamespacedName, scaler)

	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Unable to find scaler object")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Scaler invoke error")
		return ctrl.Result{}, err
	}

	log.Info(fmt.Sprintf("Context : %v ", ctx))
	log.Info(fmt.Sprintf("Namespace name : %v ", req.NamespacedName))
	log.Info(fmt.Sprintf("Scaler object : %v ", scaler))

	startTime := scaler.Spec.Start
	endTime := scaler.Spec.End
	replicas := scaler.Spec.Replicas

	currentHour := time.Now().UTC().Hour()
	log.Info(fmt.Sprintf("Current hour in UTC : %d\n", currentHour))

	if currentHour >= startTime && currentHour <= endTime {
		err = scaleDeploy(scaler, r, ctx, req, replicas)

		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{RequeueAfter: time.Duration(30 * time.Second)}, nil
}

func scaleDeploy(scaler *apiv1alpha1.Scaler, r *ScalerReconciler, ctx context.Context, req ctrl.Request, replicas int32) error {
	log := logger.WithValues("Request.Namespace", req.Namespace, "Request.Name", req.Name)

	for _, deploy := range scaler.Spec.Deployments {
		deployment := &v1.Deployment{}

		err := r.Get(ctx, types.NamespacedName{
			Namespace: deploy.Namespace,
			Name:      deploy.Name,
		},
			deployment,
		)

		log.Info(fmt.Sprintf("Deployment %v", deployment))
		if err != nil {
			log.Info("Issue here")
			return err
		}

		if deployment.Spec.Replicas != &replicas {
			deployment.Spec.Replicas = &replicas
			err := r.Update(ctx, deployment)

			if err != nil {
				scaler.Status.Status = apiv1alpha1.FAILED
				return err
			}

			scaler.Status.Status = apiv1alpha1.SUCCESS

			err = r.Status().Update(ctx, scaler)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *ScalerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&apiv1alpha1.Scaler{}).
		Complete(r)
}
