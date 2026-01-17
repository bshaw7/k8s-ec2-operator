/*
Copyright 2026.

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

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ec2v1alpha1 "github.com/bshaw7/k8s-ec2-operator/api/v1alpha1"
)

// EC2InstanceReconciler reconciles a EC2Instance object
type EC2InstanceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=ec2.my.domain,resources=ec2instances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ec2.my.domain,resources=ec2instances/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ec2.my.domain,resources=ec2instances/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the EC2Instance object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.22.4/pkg/reconcile
func (r *EC2InstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// 1. Fetch the EC2Instance object
	ec2Instance := &ec2v1alpha1.EC2Instance{}
	err := r.Get(ctx, req.NamespacedName, ec2Instance)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. Check if the instance is already created
	if ec2Instance.Status.InstanceID != "" {
		// If it exists, we stop.
		// NOTE: In a real operator, we would check if tags match here and update them if needed!
		return ctrl.Result{}, nil
	}

	log.Info("Creating a new EC2 Instance...", "AMI", ec2Instance.Spec.AMI)

	// --- AWS LOGIC ---

	// A. Load Config
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("ap-south-1"))
	if err != nil {
		log.Error(err, "Failed to load AWS config")
		return ctrl.Result{}, err
	}

	ec2Client := ec2.NewFromConfig(cfg)

	// --- NEW: Prepare Tags ---
	// We must loop through the map from your YAML and convert it to AWS format
	var awsTags []types.Tag
	for key, value := range ec2Instance.Spec.Tags {
		awsTags = append(awsTags, types.Tag{
			Key:   aws.String(key),
			Value: aws.String(value),
		})
	}
	// -------------------------

	// B. Prepare Input (Now with TagSpecifications!)
	runInput := &ec2.RunInstancesInput{
		ImageId:      aws.String(ec2Instance.Spec.AMI),
		InstanceType: types.InstanceType(ec2Instance.Spec.InstanceType),
		SubnetId:     aws.String(ec2Instance.Spec.SubnetID),
		MinCount:     aws.Int32(1),
		MaxCount:     aws.Int32(1),
		// This tells AWS: "Apply these tags to the instance resource we are creating"
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeInstance,
				Tags:         awsTags,
			},
		},
	}

	// C. Call AWS
	result, err := ec2Client.RunInstances(ctx, runInput)
	if err != nil {
		log.Error(err, "Failed to create EC2 instance")
		return ctrl.Result{}, err
	}

	newInstanceID := *result.Instances[0].InstanceId
	log.Info("Successfully created EC2 Instance!", "ID", newInstanceID)

	// 3. Update Status
	ec2Instance.Status.InstanceID = newInstanceID
	ec2Instance.Status.State = "running"

	err = r.Status().Update(ctx, ec2Instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *EC2InstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ec2v1alpha1.EC2Instance{}).
		Named("ec2instance").
		Complete(r)
}
