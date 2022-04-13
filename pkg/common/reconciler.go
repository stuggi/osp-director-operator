/*
Copyright 2020 Red Hat

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

package common

import (
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"

	//	"github.com/openstack-k8s-operators/osp-director-operator/api/shared"
	//	ospdirectorv1beta1 "github.com/openstack-k8s-operators/osp-director-operator/api/v1beta1"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ReconcilerCommon - common reconciler interface
type ReconcilerCommon interface {
	GetClient() client.Client
	GetKClient() kubernetes.Interface
	GetLogger() logr.Logger
	GetScheme() *runtime.Scheme
}

// InstanceCommon - common OSP-D resource instance interface
type InstanceCommon interface {
	// Place anything we want from "inherited" (metav1 types, etc) funcs here
	GetName() string
	GetNamespace() string

	// Place our types' custom funcs here
	IsReady() bool
}
