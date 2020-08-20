// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package tests

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/talos-systems/sidero/app/metal-controller-manager/api/v1alpha1"
	"github.com/talos-systems/sidero/app/metal-controller-manager/pkg/client"
	"github.com/talos-systems/talos/pkg/retry"
)

// TestServerRegistration verifies that all the servers got registered.
func (suite *TestSuite) TestServerRegistration(t *testing.T) {
	config, err := suite.clusterAPI.GetRestConfig(suite.ctx)
	require.NoError(t, err)

	metalClient, err := client.NewClient(config)
	require.NoError(t, err)

	var servers *v1alpha1.ServerList

	// wait for all the servers to be registered
	require.NoError(t, retry.Constant(3*time.Minute, retry.WithUnits(10*time.Second)).Retry(func() error {
		servers = &v1alpha1.ServerList{}

		if err = metalClient.List(suite.ctx, servers); err != nil {
			return retry.UnexpectedError(err)
		}

		if len(servers.Items) != suite.options.Nodes {
			return retry.ExpectedError(fmt.Errorf("%d != %d", len(servers.Items), suite.options.Nodes))
		}

		return nil
	}))

	assert.Len(t, servers.Items, suite.options.Nodes)

	nodes := suite.bootstrapCluster.Nodes()
	expectedUUIDs := make([]string, len(nodes))

	for i := range nodes {
		expectedUUIDs[i] = nodes[i].UUID.String()
	}

	actualUUIDs := make([]string, len(servers.Items))

	for i := range servers.Items {
		actualUUIDs[i] = servers.Items[i].Name
	}

	sort.Strings(expectedUUIDs)
	sort.Strings(actualUUIDs)

	assert.Equal(t, expectedUUIDs, actualUUIDs)
}
