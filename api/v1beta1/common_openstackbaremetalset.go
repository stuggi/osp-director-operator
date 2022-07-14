package v1beta1

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetBMSetByName finds and return a OSBMSet object using the specified params.
func GetBMSetByName(ctx context.Context, c client.Client, namespace, name string) (*OpenStackBaremetalSet, error) {
	bmset := &OpenStackBaremetalSet{}
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	if err := c.Get(ctx, key, bmset); err != nil {
		return nil, fmt.Errorf(fmt.Sprintf("failed to get OpenStackBaremetalSet/%s - %s", name, err.Error()))
	}

	return bmset, nil
}
