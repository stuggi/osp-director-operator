package v1beta1

import (
	"context"
	"fmt"

	v1 "k8s.io/api/apps/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

//
// AddOSNetConfigRefLabel - add osnetcfg CR label reference which is used in
// the in the osnetcfg controller to watch this resource and reconcile
//
func AddOSNetConfigRefLabel(
	namespace string,
	subnetName string,
	labels map[string]string,
) (map[string]string, error) {

	//
	// Get OSnet with SubNetNameLabelSelector: subnetName
	//
	labelSelector := map[string]string{
		SubNetNameLabelSelector: subnetName,
	}
	osnet, err := GetOpenStackNetWithLabel(namespace, labelSelector)
	if err != nil && k8s_errors.IsNotFound(err) {
		return labels, fmt.Errorf(fmt.Sprintf("OpenStackNet %s not found reconcile again in 10 seconds", subnetName))
	} else if err != nil {
		return labels, fmt.Errorf(fmt.Sprintf("Failed to get OpenStackNet %s ", subnetName))
	}

	//
	// get ownerReferences entry with Kind OpenStackNetConfig
	//
	for _, ownerRef := range osnet.ObjectMeta.OwnerReferences {
		if ownerRef.Kind == "OpenStackNetConfig" {
			//
			// merge with obj labels
			//
			labels = MergeStringMaps(
				labels,
				map[string]string{
					OpenStackNetConfigReconcileLabel: ownerRef.Name,
				},
			)

			break
		}
	}

	return labels, nil
}

// GetOpenStackNetsWithLabel - Return a list of all OpenStackNets in the namespace that have (optional) labels
func GetOpenStackNetsWithLabel(
	namespace string,
	labelSelector map[string]string,
) (*OpenStackNetList, error) {
	osNetList := &OpenStackNetList{}

	listOpts := []client.ListOption{
		client.InNamespace(namespace),
	}

	if len(labelSelector) > 0 {
		labels := client.MatchingLabels(labelSelector)
		listOpts = append(listOpts, labels)
	}

	if err := webhookClient.List(context.TODO(), osNetList, listOpts...); err != nil {
		return nil, err
	}

	return osNetList, nil
}

// GetOpenStackNetWithLabel - Return OpenStackNet with labels
func GetOpenStackNetWithLabel(
	namespace string,
	labelSelector map[string]string,
) (*OpenStackNet, error) {

	osNetList, err := GetOpenStackNetsWithLabel(
		namespace,
		labelSelector,
	)
	if err != nil {
		return nil, err
	}
	if len(osNetList.Items) == 0 {
		return nil, k8s_errors.NewNotFound(v1.Resource("openstacknet"), fmt.Sprint(labelSelector))
	} else if len(osNetList.Items) > 1 {
		return nil, fmt.Errorf("multiple OpenStackNet with label %v not found", labelSelector)
	}
	return &osNetList.Items[0], nil
}

// GetOpenStackNetsMapWithLabel - Return a map[NameLower] of all OpenStackNets in the namespace that have (optional) labels
func GetOpenStackNetsMapWithLabel(
	namespace string,
	labelSelector map[string]string,
) (map[string]OpenStackNet, error) {
	osNetList, err := GetOpenStackNetsWithLabel(
		namespace,
		labelSelector,
	)
	if err != nil {
		return nil, err
	}

	osNetMap := map[string]OpenStackNet{}
	for _, osNet := range osNetList.Items {
		osNetMap[osNet.Spec.NameLower] = osNet
	}

	return osNetMap, nil
}
