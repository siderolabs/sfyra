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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

const serverClassName = "default"

// TestServerClassDefault verifies server class creation.
func (suite *TestSuite) TestServerClassDefault(t *testing.T) {
	config, err := suite.clusterAPI.GetRestConfig(suite.ctx)
	require.NoError(t, err)

	metalClient, err := client.NewClient(config)
	require.NoError(t, err)

	var serverClass v1alpha1.ServerClass

	if err = metalClient.Get(suite.ctx, types.NamespacedName{Name: serverClassName}, &serverClass); err != nil {
		if !apierrors.IsNotFound(err) {
			require.NoError(t, err)
		}

		serverClass.APIVersion = "metal.sidero.dev/v1alpha1"
		serverClass.Name = serverClassName
		serverClass.Spec.Qualifiers.CPU = append(serverClass.Spec.Qualifiers.CPU, v1alpha1.CPUInformation{
			Manufacturer: "QEMU",
			Version:      "pc-q35-4.2",
		})

		require.NoError(t, metalClient.Create(suite.ctx, &serverClass))
	}

	// wait for the server class to gather all nodes (all nodes should match)
	require.NoError(t, retry.Constant(2*time.Minute, retry.WithUnits(10*time.Second)).Retry(func() error {
		if err = metalClient.Get(suite.ctx, types.NamespacedName{Name: serverClassName}, &serverClass); err != nil {
			return retry.UnexpectedError(err)
		}

		if len(serverClass.Status.ServersAvailable) != suite.options.Nodes {
			return retry.ExpectedError(fmt.Errorf("%d != %d", len(serverClass.Status.ServersAvailable), suite.options.Nodes))
		}

		return nil
	}))

	assert.Len(t, serverClass.Status.ServersAvailable, suite.options.Nodes)

	nodes := suite.bootstrapCluster.Nodes()
	expectedUUIDs := make([]string, len(nodes))

	for i := range nodes {
		expectedUUIDs[i] = nodes[i].UUID.String()
	}

	actualUUIDs := append([]string(nil), serverClass.Status.ServersAvailable...)

	sort.Strings(expectedUUIDs)
	sort.Strings(actualUUIDs)

	assert.Equal(t, expectedUUIDs, actualUUIDs)
}
