// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package infrastructure

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client"
)

// SideroOptions sidero provider options.
type SideroOptions struct {
	ManagerAPIEndpoint       net.IP
	ServerRebootTimeout      time.Duration
	TestPowerExplicitFailure float64
	TestPowerSilentFailure   float64
	ManagerHostNetwork       bool
}

// SideroProvider infrastructure provider.
type SideroProvider struct{}

// Init sets up infrastructure provider.
//nolint:errcheck
func (s *SideroProvider) Init(ctx context.Context, client client.Client, clientset *kubernetes.Clientset, opts *Options) error {
	os.Setenv("SIDERO_CONTROLLER_MANAGER_HOST_NETWORK", fmt.Sprintf("%t", opts.ManagerHostNetwork))
	os.Setenv("SIDERO_CONTROLLER_MANAGER_API_ENDPOINT", opts.ManagerAPIEndpoint.String())
	os.Setenv("SIDERO_CONTROLLER_MANAGER_SERVER_REBOOT_TIMEOUT", opts.ServerRebootTimeout.String()) // wiping/reboot is fast in the test environment
	os.Setenv("SIDERO_CONTROLLER_MANAGER_TEST_POWER_EXPLICIT_FAILURE", fmt.Sprintf("%f", opts.TestPowerExplicitFailure))
	os.Setenv("SIDERO_CONTROLLER_MANAGER_TEST_POWER_SILENT_FAILURE", fmt.Sprintf("%f", opts.TestPowerSilentFailure))

	_, err := clientset.CoreV1().Namespaces().Get(ctx, "sidero-system", metav1.GetOptions{})
	if err != nil {
		_, err = client.Init(opts.InitOptions)

		return err
	}

	return nil
}
