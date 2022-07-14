package v1beta2

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetVMSetByName finds and return a OSVMSet object using the specified params.
func GetVMSetByName(ctx context.Context, c client.Client, namespace, name string) (*OpenStackVMSet, error) {
	vmset := &OpenStackVMSet{}
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	if err := c.Get(ctx, key, vmset); err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("failed to get OpenStackVMSet/%s - %s", name, err.Error()))
	}

	return vmset, nil
}
