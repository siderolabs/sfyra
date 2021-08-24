// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package capi manages CAPI installation, provides default client for CAPI CRDs.
package capi

import (
	"context"
	"io/ioutil"
	"time"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/talos-systems/sfyra/pkg/capi/infrastructure"
	"github.com/talos-systems/sfyra/pkg/talos"
)

// Manager installs and controls cluster API installation.
type Manager struct {
	cluster talos.Cluster

	kubeconfig    client.Kubeconfig
	client        client.Client
	clientset     *kubernetes.Clientset
	runtimeClient runtimeclient.Client

	options Options
}

// Options for the CAPI installer.
type Options struct {
	ClusterctlConfigPath    string
	CoreProvider            string
	BootstrapProviders      []string
	InfrastructureProviders []string
	ControlPlaneProviders   []string

	PowerSimulatedExplicitFailureProb float64
	PowerSimulatedSilentFailureProb   float64
}

// NewManager creates new Manager object.
func NewManager(ctx context.Context, cluster talos.Cluster, options Options) (*Manager, error) {
	clusterAPI := &Manager{
		options: options,
		cluster: cluster,
	}

	var err error

	clusterAPI.client, err = client.New(options.ClusterctlConfigPath)
	if err != nil {
		return nil, err
	}

	clusterAPI.clientset, err = clusterAPI.cluster.KubernetesClient().K8sClient(ctx)
	if err != nil {
		return nil, err
	}

	return clusterAPI, nil
}

// GetKubeconfig returns kubeconfig in clusterctl expected format.
func (clusterAPI *Manager) GetKubeconfig(ctx context.Context) (client.Kubeconfig, error) {
	if clusterAPI.kubeconfig.Path != "" {
		return clusterAPI.kubeconfig, nil
	}

	kubeconfigBytes, err := clusterAPI.cluster.KubernetesClient().Kubeconfig(ctx)
	if err != nil {
		return client.Kubeconfig{}, err
	}

	tmpFile, err := ioutil.TempFile("", "kubeconfig")
	if err != nil {
		return client.Kubeconfig{}, err
	}

	_, err = tmpFile.Write(kubeconfigBytes)
	if err != nil {
		return client.Kubeconfig{}, err
	}

	clusterAPI.kubeconfig.Path = tmpFile.Name()
	clusterAPI.kubeconfig.Context = "admin@" + clusterAPI.cluster.Name()

	return clusterAPI.kubeconfig, nil
}

// GetManagerClient client returns instance of cluster API client.
func (clusterAPI *Manager) GetManagerClient() client.Client {
	return clusterAPI.client
}

// GetClient returns k8s client stuffed with CAPI CRDs.
func (clusterAPI *Manager) GetClient(ctx context.Context) (runtimeclient.Client, error) {
	if clusterAPI.runtimeClient != nil {
		return clusterAPI.runtimeClient, nil
	}

	config, err := clusterAPI.cluster.KubernetesClient().K8sRestConfig(ctx)
	if err != nil {
		return nil, err
	}

	clusterAPI.runtimeClient, err = GetMetalClient(config)

	return clusterAPI.runtimeClient, err
}

// Install the Manager components and wait for them to be ready.
func (clusterAPI *Manager) Install(ctx context.Context) error {
	kubeconfig, err := clusterAPI.GetKubeconfig(ctx)
	if err != nil {
		return err
	}

	opts := infrastructure.Options{
		SideroOptions: infrastructure.SideroOptions{
			ManagerHostNetwork:       true,
			ManagerAPIEndpoint:       clusterAPI.cluster.SideroComponentsIP(),
			ServerRebootTimeout:      time.Second * 30,
			TestPowerExplicitFailure: clusterAPI.options.PowerSimulatedExplicitFailureProb,
			TestPowerSilentFailure:   clusterAPI.options.PowerSimulatedSilentFailureProb,
		},
		InitOptions: client.InitOptions{
			Kubeconfig:              kubeconfig,
			CoreProvider:            clusterAPI.options.CoreProvider,
			BootstrapProviders:      clusterAPI.options.BootstrapProviders,
			ControlPlaneProviders:   clusterAPI.options.ControlPlaneProviders,
			InfrastructureProviders: clusterAPI.options.InfrastructureProviders,
			TargetNamespace:         "",
			WatchingNamespace:       "",
			LogUsageInstructions:    false,
		},
	}

	for _, kind := range clusterAPI.options.InfrastructureProviders {
		provider, err := infrastructure.NewProvider(kind)
		if err != nil {
			return err
		}

		if err = provider.Init(ctx, clusterAPI.client, clusterAPI.clientset, &opts); err != nil {
			return err
		}
	}

	return nil
}
