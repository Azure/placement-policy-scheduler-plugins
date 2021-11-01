// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/Azure/placement-policy-scheduler-plugins/apis/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// PlacementPolicyLister helps list PlacementPolicies.
// All objects returned here must be treated as read-only.
type PlacementPolicyLister interface {
	// List lists all PlacementPolicies in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.PlacementPolicy, err error)
	// PlacementPolicies returns an object that can list and get PlacementPolicies.
	PlacementPolicies(namespace string) PlacementPolicyNamespaceLister
	PlacementPolicyListerExpansion
}

// placementPolicyLister implements the PlacementPolicyLister interface.
type placementPolicyLister struct {
	indexer cache.Indexer
}

// NewPlacementPolicyLister returns a new PlacementPolicyLister.
func NewPlacementPolicyLister(indexer cache.Indexer) PlacementPolicyLister {
	return &placementPolicyLister{indexer: indexer}
}

// List lists all PlacementPolicies in the indexer.
func (s *placementPolicyLister) List(selector labels.Selector) (ret []*v1alpha1.PlacementPolicy, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.PlacementPolicy))
	})
	return ret, err
}

// PlacementPolicies returns an object that can list and get PlacementPolicies.
func (s *placementPolicyLister) PlacementPolicies(namespace string) PlacementPolicyNamespaceLister {
	return placementPolicyNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// PlacementPolicyNamespaceLister helps list and get PlacementPolicies.
// All objects returned here must be treated as read-only.
type PlacementPolicyNamespaceLister interface {
	// List lists all PlacementPolicies in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*v1alpha1.PlacementPolicy, err error)
	// Get retrieves the PlacementPolicy from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*v1alpha1.PlacementPolicy, error)
	PlacementPolicyNamespaceListerExpansion
}

// placementPolicyNamespaceLister implements the PlacementPolicyNamespaceLister
// interface.
type placementPolicyNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all PlacementPolicies in the indexer for a given namespace.
func (s placementPolicyNamespaceLister) List(selector labels.Selector) (ret []*v1alpha1.PlacementPolicy, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.PlacementPolicy))
	})
	return ret, err
}

// Get retrieves the PlacementPolicy from the indexer for a given namespace and name.
func (s placementPolicyNamespaceLister) Get(name string) (*v1alpha1.PlacementPolicy, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1alpha1.Resource("placementpolicy"), name)
	}
	return obj.(*v1alpha1.PlacementPolicy), nil
}
