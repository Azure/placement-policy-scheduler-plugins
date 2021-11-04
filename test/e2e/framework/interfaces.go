//go:build e2e
// +build e2e

// Vendored from Azure/secrets-store-csi-driver-provider-azure
//  * tag: v1.0.0-rc.0,
//  * link: https://github.com/Azure/secrets-store-csi-driver-provider-azure/blob/b458c5cd05/test/e2e/framework/interfaces.go

package framework

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Interfaces to scope down client.Client

// Getter can get resources.
type Getter interface {
	Get(ctx context.Context, key client.ObjectKey, obj client.Object) error
}

// Creator can create resources.
type Creator interface {
	Create(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
}

// Lister can list resources.
type Lister interface {
	List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error
}

// Deleter can delete resources.
type Deleter interface {
	Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error
}

// Updater can update resources.
type Updater interface {
	Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error
}

// GetLister can get and list resources.
type GetLister interface {
	Getter
	Lister
}
