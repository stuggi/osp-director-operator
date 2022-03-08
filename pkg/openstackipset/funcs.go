package openstackipset

import (
	"context"
	"fmt"
	"strings"
	"time"

	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/osp-director-operator/pkg/common"
	controlplane "github.com/openstack-k8s-operators/osp-director-operator/pkg/controlplane"
	openstacknetconfig "github.com/openstack-k8s-operators/osp-director-operator/pkg/openstacknetconfig"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

//
// EnsureIPs - Creates IPSet and verify/wait for IPs created
//
func EnsureIPs(
	r common.ReconcilerCommon,
	obj client.Object,
	cond *ospdirectorv1beta1.Condition,
	name string,
	networks []string,
	hostCount int,
	vip bool,
	serviceVIP bool,
	deletedHosts []string,
	addToPredictableIPs bool,
) (map[string]ospdirectorv1beta1.HostStatus, reconcile.Result, error) {

	status := map[string]ospdirectorv1beta1.HostStatus{}

	//
	// create IPSet to request IPs for all networks
	//
	_, err := createOrUpdateIPSet(
		r,
		obj,
		cond,
		name,
		networks,
		hostCount,
		vip,
		serviceVIP,
		deletedHosts,
		addToPredictableIPs,
	)
	if err != nil {
		return status, reconcile.Result{}, err
	}

	//
	// get ipset and verify all IPs got created
	//
	ipSet := &ospdirectorv1beta1.OpenStackIPSet{}
	err = r.GetClient().Get(context.TODO(), types.NamespacedName{
		Name:      strings.ToLower(name),
		Namespace: obj.GetNamespace()},
		ipSet)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to get %s %s ", ipSet.Kind, ipSet.Name)
		cond.Reason = ospdirectorv1beta1.IPSetCondReasonError
		cond.Type = ospdirectorv1beta1.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, obj, err)

		return status, reconcile.Result{}, err
	}

	//
	// check for all hosts got created on the IPset
	//
	if len(ipSet.Status.Hosts) < hostCount {
		cond.Message = fmt.Sprintf("Waiting on hosts to be created on IPSet %v - %v", len(ipSet.Status.Hosts), hostCount)
		cond.Reason = ospdirectorv1beta1.IPSetCondReasonWaitingOnHosts
		cond.Type = ospdirectorv1beta1.CommonCondTypeWaiting

		return status, reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

	//
	// check if hosts got deleted
	//
	if len(ipSet.Status.Hosts) > hostCount &&
		(len(ipSet.Status.Hosts)-len(deletedHosts)) == hostCount {
		for _, hostname := range deletedHosts {
			delete(ipSet.Status.Hosts, hostname)
		}

	}

	//
	// get OSNetCfg object
	//
	osnetcfg := &ospdirectorv1beta1.OpenStackNetConfig{}
	err = r.GetClient().Get(context.TODO(), types.NamespacedName{
		Name:      strings.ToLower(obj.GetLabels()[openstacknetconfig.OpenStackNetConfigReconcileLabel]),
		Namespace: obj.GetNamespace()},
		osnetcfg)
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to get %s %s ", osnetcfg.Kind, osnetcfg.Name)
		cond.Reason = ospdirectorv1beta1.NetConfigCondReasonnError
		cond.Type = ospdirectorv1beta1.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, obj, err)

		return status, reconcile.Result{}, err
	}

	for hostname, hostStatus := range ipSet.Status.Hosts {
		err = openstacknetconfig.WaitOnIPsCreated(
			r,
			obj,
			cond,
			osnetcfg,
			networks,
			hostname,
			&hostStatus,
		)
		if err != nil {
			return status, reconcile.Result{RequeueAfter: 10 * time.Second}, nil
		}

		status[hostname] = hostStatus
	}

	return status, reconcile.Result{}, nil
}

//
// createOrUpdateIPSet - Creates or updates IPSet
//
func createOrUpdateIPSet(
	r common.ReconcilerCommon,
	obj client.Object,
	cond *ospdirectorv1beta1.Condition,
	name string,
	networks []string,
	hostCount int,
	vip bool,
	serviceVIP bool,
	deletedHosts []string,
	addToPredictableIPs bool,
) (*ospdirectorv1beta1.OpenStackIPSet, error) {
	ipSet := &ospdirectorv1beta1.OpenStackIPSet{
		ObjectMeta: metav1.ObjectMeta{
			// use the role name as the VM CR name
			Name:      strings.ToLower(name),
			Namespace: obj.GetNamespace(),
		},
	}

	op, err := controllerutil.CreateOrUpdate(context.TODO(), r.GetClient(), ipSet, func() error {
		ipSet.Labels = common.MergeStringMaps(
			ipSet.Labels,
			common.GetLabels(obj, controlplane.AppLabel, map[string]string{}),
		)

		ipSet.Spec.HostCount = hostCount
		ipSet.Spec.Networks = networks
		ipSet.Spec.RoleName = name
		ipSet.Spec.VIP = vip
		ipSet.Spec.ServiceVIP = serviceVIP
		ipSet.Spec.DeletedHosts = deletedHosts
		ipSet.Spec.AddToPredictableIPs = addToPredictableIPs

		// TODO: (mschupppert) move to webhook
		if len(networks) == 0 {
			ipSet.Spec.Networks = []string{"ctlplane"}
		}

		err := controllerutil.SetControllerReference(obj, ipSet, r.GetScheme())
		if err != nil {
			cond.Message = fmt.Sprintf("Error set controller reference for %s %s", ipSet.Kind, ipSet.Name)
			cond.Reason = ospdirectorv1beta1.CommonCondReasonControllerReferenceError
			cond.Type = ospdirectorv1beta1.CommonCondTypeError
			err = common.WrapErrorForObject(cond.Message, obj, err)

			return err
		}

		return nil
	})
	if err != nil {
		cond.Message = fmt.Sprintf("Failed to create or update %s %s ", ipSet.Kind, ipSet.Name)
		cond.Reason = ospdirectorv1beta1.IPSetCondReasonError
		cond.Type = ospdirectorv1beta1.CommonCondTypeError
		err = common.WrapErrorForObject(cond.Message, obj, err)

		return ipSet, err
	}

	cond.Message = fmt.Sprintf("%s %s successfully reconciled", ipSet.Kind, ipSet.Name)
	if op != controllerutil.OperationResultNone {
		cond.Message = fmt.Sprintf("%s - operation: %s", cond.Message, string(op))

		common.LogForObject(
			r,
			cond.Message,
			obj,
		)
	}
	cond.Reason = ospdirectorv1beta1.IPSetCondReasonCreated
	cond.Type = ospdirectorv1beta1.CommonCondTypeCreated

	return ipSet, nil
}

//
// SyncIPsetStatus - sync relevant information from IPSet to CR status
//
func SyncIPsetStatus(
	cond *ospdirectorv1beta1.Condition,
	instanceStatus map[string]ospdirectorv1beta1.HostStatus,
	ipsetHostStatus ospdirectorv1beta1.HostStatus,
) ospdirectorv1beta1.HostStatus {
	var hostStatus ospdirectorv1beta1.HostStatus
	if _, ok := instanceStatus[ipsetHostStatus.Hostname]; !ok {
		hostStatus = ipsetHostStatus
	} else {
		// Note:
		// do not sync all information as other controllers are
		// the master for e.g.
		// - BMH <-> hostname mapping
		// - create of networkDataSecretName and userDataSecretName
		hostStatus = instanceStatus[ipsetHostStatus.Hostname]
		hostStatus.AnnotatedForDeletion = ipsetHostStatus.AnnotatedForDeletion
		// TODO: (mschuppert) remove CtlplaneIP where used (osbms) and replace with hostStatus.IPAddresses[<ctlplane>]
		hostStatus.CtlplaneIP = ipsetHostStatus.CtlplaneIP
		hostStatus.IPAddresses = ipsetHostStatus.IPAddresses
		hostStatus.ProvisioningState = ipsetHostStatus.ProvisioningState

		if ipsetHostStatus.HostRef != ospdirectorv1beta1.HostRefInitState {
			hostStatus.HostRef = ipsetHostStatus.HostRef
		}
	}

	hostStatus.ProvisioningState = ospdirectorv1beta1.ProvisioningState(cond.Type)

	return hostStatus
}
