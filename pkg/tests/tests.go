// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package tests provides the Sidero tests.
package tests

import (
	"context"
	"testing"

	"github.com/talos-systems/sfyra/pkg/config"
	"github.com/talos-systems/sfyra/pkg/setup"
)

// TestSuite combines all the integration tests.
type TestSuite struct {
	ctx              context.Context
	options          *config.Options
	bootstrapCluster *setup.BootstrapCluster
	clusterAPI       *setup.ClusterAPI
}

// Run all the tests.
func Run(ctx context.Context, options *config.Options, bootstrapCluster *setup.BootstrapCluster, clusterAPI *setup.ClusterAPI) (ok bool) {
	suite := &TestSuite{
		ctx:              ctx,
		options:          options,
		bootstrapCluster: bootstrapCluster,
		clusterAPI:       clusterAPI,
	}

	return testing.MainStart(matchStringOnly(func(pat, str string) (bool, error) { return true, nil }), []testing.InternalTest{
		{
			"TestServerRegistration",
			suite.TestServerRegistration,
		},
		{
			"TestEnvironmentDefault",
			suite.TestEnvironmentDefault,
		},
		{
			"TestServerClassDefault",
			suite.TestServerClassDefault,
		},
	}, nil, nil).Run() == 0
}
