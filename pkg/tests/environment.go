// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/talos-systems/go-procfs/procfs"
	"github.com/talos-systems/go-retry/retry"
	"github.com/talos-systems/sidero/app/metal-controller-manager/api/v1alpha1"
	"github.com/talos-systems/sidero/app/metal-controller-manager/pkg/client"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

const environmentName = "default"

// TestEnvironmentDefault verifies default environment creation.
func (suite *TestSuite) TestEnvironmentDefault(t *testing.T) {
	config, err := suite.clusterAPI.GetRestConfig(suite.ctx)
	require.NoError(t, err)

	metalClient, err := client.NewClient(config)
	require.NoError(t, err)

	var environment v1alpha1.Environment

	if err = metalClient.Get(suite.ctx, types.NamespacedName{Name: environmentName}, &environment); err != nil {
		if !apierrors.IsNotFound(err) {
			require.NoError(t, err)
		}

		cmdline := procfs.NewDefaultCmdline()
		cmdline.Append("console", "ttyS0")
		cmdline.Append("reboot", "k")
		cmdline.Append("panic", "1")
		cmdline.Append("talos.platform", "metal")
		cmdline.Append("talos.config", fmt.Sprintf("http://%s:9091/configdata?uuid=", suite.bootstrapCluster.MasterIP()))

		environment.APIVersion = "metal.sidero.dev/v1alpha1"
		environment.Name = environmentName
		environment.Spec.Kernel.URL = suite.options.TalosKernelURL
		environment.Spec.Kernel.SHA512 = "" // TODO: add a test
		environment.Spec.Kernel.Args = cmdline.Strings()
		environment.Spec.Initrd.URL = suite.options.TalosInitrdURL
		environment.Spec.Initrd.SHA512 = "" // TODO: add a test

		require.NoError(t, metalClient.Create(suite.ctx, &environment))
	}

	// wait for the environment to report ready
	require.NoError(t, retry.Constant(5*time.Minute, retry.WithUnits(10*time.Second)).Retry(func() error {
		if err = metalClient.Get(suite.ctx, types.NamespacedName{Name: environmentName}, &environment); err != nil {
			return retry.UnexpectedError(err)
		}

		assetURLs := map[string]struct{}{
			suite.options.TalosKernelURL: {},
			suite.options.TalosInitrdURL: {},
		}

		for _, cond := range environment.Status.Conditions {
			if cond.Status == "True" && cond.Type == "Ready" {
				delete(assetURLs, cond.URL)
			}
		}

		if len(assetURLs) > 0 {
			return retry.ExpectedError(fmt.Errorf("some assets are not ready: %v", assetURLs))
		}

		return nil
	}))
}
