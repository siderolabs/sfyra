// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package infrastructure contains infrastructure providers setup hooks and ready conditions.
package infrastructure

import (
	"context"
	"fmt"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client"
)

// Provider defines an interface for the infrastructure provider.
type Provider interface {
	Init(context.Context, client.Client, *kubernetes.Clientset, *Options) error
}

const (
	// AWS infrastructure provider.
	AWS = "aws"
	// Sidero infrastructure provider.
	Sidero = "sidero"
)

// NewProvider creates a new infrastructure provider of the specified kind.
func NewProvider(kind string) (Provider, error) {
	switch kind {
	case AWS:
		// TODO: implement it
	case Sidero:
		return &SideroProvider{}, nil
	}

	return nil, fmt.Errorf("failed to set up unknown provider kind %v", kind)
}
