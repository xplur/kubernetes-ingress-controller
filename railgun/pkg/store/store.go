package store

import (
	"context"
	"fmt"

	configurationv1 "github.com/kong/kubernetes-ingress-controller/pkg/apis/configuration/v1"
	configurationv1beta1 "github.com/kong/kubernetes-ingress-controller/pkg/apis/configuration/v1beta1"
	oldstr "github.com/kong/kubernetes-ingress-controller/pkg/store"
	"github.com/kong/kubernetes-ingress-controller/railgun/apis/configuration/v1alpha1"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	knative "knative.dev/networking/pkg/apis/networking/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// -----------------------------------------------------------------------------
// Secret Controller - Storer - Public Functions
// -----------------------------------------------------------------------------

// New produces a new oldstr.Storer which will house the provided kubernetes.Clientset
// and will provide parsing and translation of Kubernetes objects to the Kong Admin DSL.
//
// TODO: there's significant technical debt associated with the storer implementations and
// interface as a whole, we need to determine if and how we want to continue using the store
// package in the future. Perhaps we can just provide a kubernetes.Clientset later on?
// If we continue to use storer, we should consider expanding the interface to include
// contexts, as all the relevant API calls made underneath the hood here use contexts.
func New(c client.Client) oldstr.Storer {
	return &store{c}
}

// -----------------------------------------------------------------------------
// Secret Controller - Storer - Private Types
// -----------------------------------------------------------------------------

type store struct {
	c client.Client
}

// -----------------------------------------------------------------------------
// Secret Controller - Storer - Public Get Methods
// -----------------------------------------------------------------------------

func (s *store) GetSecret(namespace, name string) (*apiv1.Secret, error) {
	var secret *apiv1.Secret
	if err := s.c.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: name}, secret); err != nil {
		return nil, err
	}
	return secret, nil
}

func (s *store) GetService(namespace, name string) (*apiv1.Service, error) {
	var service *apiv1.Service
	if err := s.c.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: name}, service); err != nil {
		return nil, err
	}
	return service, nil
}

func (s *store) GetEndpointsForService(namespace, name string) (*apiv1.Endpoints, error) {
	var endpoints *apiv1.Endpoints
	if err := s.c.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: name}, endpoints); err != nil {
		return nil, err
	}
	return endpoints, nil
}

func (s *store) GetKongIngress(namespace, name string) (*configurationv1.KongIngress, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (s *store) GetKongPlugin(namespace, name string) (*configurationv1.KongPlugin, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (s *store) GetKongClusterPlugin(name string) (*configurationv1.KongClusterPlugin, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (s *store) GetKongConsumer(namespace, name string) (*configurationv1.KongConsumer, error) {
	return nil, fmt.Errorf("unimplemented")
}

// -----------------------------------------------------------------------------
// Secret Controller - Storer - Public List Methods
// -----------------------------------------------------------------------------

func (s *store) ListIngressesV1beta1() []*networkingv1beta1.Ingress {
	var list *networkingv1beta1.IngressList
	if err := s.c.List(context.Background(), list); err != nil {
		return nil
	}

	ingresses := make([]*networkingv1beta1.Ingress, 0, len(list.Items))
	for _, ingress := range list.Items {
		ingresses = append(ingresses, &ingress)
	}

	return ingresses
}

func (s *store) ListIngressesV1() []*networkingv1.Ingress {
	var list *networkingv1.IngressList
	if err := s.c.List(context.Background(), list); err != nil {
		return nil
	}

	ingresses := make([]*networkingv1.Ingress, 0, len(list.Items))
	for _, ingress := range list.Items {
		ingresses = append(ingresses, &ingress)
	}

	return ingresses

}

func (s *store) ListTCPIngresses() ([]*configurationv1beta1.TCPIngress, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (s *store) ListUDPIngresses() ([]*v1alpha1.UDPIngress, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (s *store) ListKnativeIngresses() ([]*knative.Ingress, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (s *store) ListGlobalKongPlugins() ([]*configurationv1.KongPlugin, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (s *store) ListGlobalKongClusterPlugins() ([]*configurationv1.KongClusterPlugin, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (s *store) ListKongConsumers() []*configurationv1.KongConsumer {
	panic(fmt.Errorf("unimplemented"))
}

func (s *store) ListCACerts() ([]*apiv1.Secret, error) {
	return nil, fmt.Errorf("unimplemented")
}
