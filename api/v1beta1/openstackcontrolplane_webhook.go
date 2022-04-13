/*


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

// Generated by:
//
// operator-sdk create webhook --group osp-director --version v1beta1 --kind OpenStackControlPlane --programmatic-validation
//

package v1beta1

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

// OpenStackControlPlaneDefaults -
type OpenStackControlPlaneDefaults struct {
	OpenStackRelease string
}

var openstackControlPlaneDefaults OpenStackControlPlaneDefaults

// log is for logging in this package.
var controlplanelog = logf.Log.WithName("controlplane-resource")

// SetupWebhookWithManager - register this webhook with the controller manager
func (r *OpenStackControlPlane) SetupWebhookWithManager(mgr ctrl.Manager, defaults OpenStackControlPlaneDefaults) error {

	openstackControlPlaneDefaults = defaults

	if webhookClient == nil {
		webhookClient = mgr.GetClient()
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:verbs=create;update;delete,path=/validate-osp-director-openstack-org-v1beta1-openstackcontrolplane,mutating=false,failurePolicy=fail,sideEffects=None,groups=osp-director.openstack.org,resources=openstackcontrolplanes,versions=v1beta1,name=vopenstackcontrolplane.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &OpenStackControlPlane{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackControlPlane) ValidateCreate() error {
	controlplanelog.Info("validate create", "name", r.Name)

	if err := checkBackupOperationBlocksAction(r.Namespace, shared.APIActionCreate); err != nil {
		return err
	}

	//
	// validate OSP version, right now only 16.2/train and 17.0/wallaby are supported
	//
	if r.Spec.OpenStackRelease != "" {
		var err error
		if r.Status.OSPVersion, err = GetOSPVersion(r.Spec.OpenStackRelease); err != nil {
			return err
		}
	}

	controlPlaneList := &OpenStackControlPlaneList{}

	listOpts := []client.ListOption{
		client.InNamespace(r.Namespace),
	}

	if err := webhookClient.List(context.TODO(), controlPlaneList, listOpts...); err != nil {
		return err
	}

	if len(controlPlaneList.Items) >= 1 {
		return fmt.Errorf("only one OpenStackControlPlane instance is supported at this time")
	}

	//
	// Fail early on create if osnetcfg ist not found
	//
	_, err := GetOsNetCfg(webhookClient, r.GetNamespace(), r.GetLabels()[shared.OpenStackNetConfigReconcileLabel])
	if err != nil {
		return fmt.Errorf(fmt.Sprintf("error getting OpenStackNetConfig %s - %s: %s",
			r.GetLabels()[shared.OpenStackNetConfigReconcileLabel],
			r.Name,
			err))
	}

	//
	// validate that for all configured subnets an osnet exists
	//
	for _, vmspec := range r.Spec.VirtualMachineRoles {
		//
		// validate that for all configured subnets an osnet exists
		//
		if err := validateNetworks(r.GetNamespace(), vmspec.Networks); err != nil {
			return err
		}
	}

	return nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackControlPlane) ValidateUpdate(old runtime.Object) error {
	controlplanelog.Info("validate update", "name", r.Name)

	//
	// validate OSP version, right now only 16.2/train and 17.0/wallaby are supported
	//
	if r.Spec.OpenStackRelease != "" {
		var err error
		if r.Status.OSPVersion, err = GetOSPVersion(r.Spec.OpenStackRelease); err != nil {
			return err
		}
	}

	//
	// validate that for all configured subnets an osnet exists
	//
	for _, vmspec := range r.Spec.VirtualMachineRoles {
		//
		// validate that for all configured subnets an osnet exists
		//
		if err := validateNetworks(r.GetNamespace(), vmspec.Networks); err != nil {
			return err
		}
	}

	return nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *OpenStackControlPlane) ValidateDelete() error {
	controlplanelog.Info("validate delete", "name", r.Name)

	return checkBackupOperationBlocksAction(r.Namespace, shared.APIActionDelete)
}

//+kubebuilder:webhook:path=/mutate-osp-director-openstack-org-v1beta1-openstackcontrolplane,mutating=true,failurePolicy=fail,sideEffects=None,groups=osp-director.openstack.org,resources=openstackcontrolplanes,verbs=create;update,versions=v1beta1,name=mopenstackcontrolplane.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &OpenStackControlPlane{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *OpenStackControlPlane) Default() {
	controlplanelog.Info("default", "name", r.Name)
	//
	// set OpenStackRelease if non provided
	//
	if r.Spec.OpenStackRelease == "" {
		r.Spec.OpenStackRelease = openstackControlPlaneDefaults.OpenStackRelease
		r.Status.OSPVersion = OSPVersion(openstackControlPlaneDefaults.OpenStackRelease)
	} else {
		r.Status.OSPVersion = OSPVersion(r.Spec.OpenStackRelease)
	}
	controlplanelog.Info(fmt.Sprintf("%s %s using OSP release %s", r.GetObjectKind().GroupVersionKind().Kind, r.Name, r.Status.OSPVersion))

	//
	// set default for AdditionalServiceVIPs if non provided in ctlplane spec
	// https://docs.openstack.org/project-deploy-guide/tripleo-docs/latest/deployment/network_v2.html#service-virtual-ips
	//
	if r.Status.OSPVersion == TemplateVersion17_0 && r.Spec.AdditionalServiceVIPs == nil {
		r.Spec.AdditionalServiceVIPs = map[string]string{
			"Redis":  "internal_api",
			"OVNDBs": "internal_api",
		}

		controlplanelog.Info(fmt.Sprintf("%s %s AdditionalServiceVIPs set %v", r.GetObjectKind().GroupVersionKind().Kind, r.Name, r.Spec.AdditionalServiceVIPs))
	}

	//
	// set OpenStackNetConfig reference label if not already there
	// Note, any rename of the osnetcfg won't be reflected
	//
	if _, ok := r.GetLabels()[shared.OpenStackNetConfigReconcileLabel]; !ok {
		var subnetName string
		for _, vmRole := range r.Spec.VirtualMachineRoles {
			subnetName = vmRole.Networks[0]
			break
		}

		labels, err := AddOSNetConfigRefLabel(
			webhookClient,
			r.Namespace,
			subnetName,
			r.DeepCopy().GetLabels(),
		)
		if err != nil {
			controlplanelog.Error(err, fmt.Sprintf("error adding OpenStackNetConfig reference label on %s - %s: %s", r.Kind, r.Name, err))
		}

		r.SetLabels(labels)
		controlplanelog.Info(fmt.Sprintf("%s %s labels set to %v", r.GetObjectKind().GroupVersionKind().Kind, r.Name, r.GetLabels()))
	}

	//
	// add labels of all networks used by this CR
	//
	vipNetList, err := CreateVIPNetworkList(webhookClient, r)
	if err != nil {
		controlplanelog.Error(err, fmt.Sprintf("error creating VIP network list: %s", err))
	}

	labels := AddOSNetNameLowerLabels(
		controlplanelog,
		r.DeepCopy().GetLabels(),
		vipNetList,
	)
	if !equality.Semantic.DeepEqual(
		labels,
		r.GetLabels(),
	) {
		r.SetLabels(labels)
		controlplanelog.Info(fmt.Sprintf("%s %s labels set to %v", r.GetObjectKind().GroupVersionKind().Kind, r.Name, r.GetLabels()))
	}
}
