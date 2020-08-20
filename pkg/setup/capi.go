// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package setup

import (
	"context"
	"fmt"
	"io/ioutil"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client"

	"github.com/talos-systems/sfyra/pkg/config"
)

// ClusterAPI installs and manages cluster API installation.
type ClusterAPI struct {
	Options *config.Options

	bootstrapCluster *BootstrapCluster

	kubeconfig client.Kubeconfig
	client     client.Client
	clientset  *kubernetes.Clientset
}

// NewClusterAPI creates new ClusterAPI object.
func NewClusterAPI(ctx context.Context, options *config.Options, bootstrapCluster *BootstrapCluster) (*ClusterAPI, error) {
	clusterAPI := &ClusterAPI{
		Options:          options,
		bootstrapCluster: bootstrapCluster,
	}

	var err error

	clusterAPI.client, err = client.New("")
	if err != nil {
		return nil, err
	}

	clusterAPI.clientset, err = clusterAPI.bootstrapCluster.Access().K8sClient(ctx)
	if err != nil {
		return nil, err
	}

	return clusterAPI, nil
}

func (clusterAPI *ClusterAPI) getKubeconfig(ctx context.Context) (client.Kubeconfig, error) {
	if clusterAPI.kubeconfig.Path != "" {
		return clusterAPI.kubeconfig, nil
	}

	talosClient, err := clusterAPI.bootstrapCluster.Access().Client()
	if err != nil {
		return client.Kubeconfig{}, err
	}

	kubeconfigBytes, err := talosClient.Kubeconfig(ctx)
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
	clusterAPI.kubeconfig.Context = "admin@" + clusterAPI.Options.BootstrapClusterName

	return clusterAPI.kubeconfig, nil
}

// GetRestConfig returns parsed Kubernetes config.
//
// TODO: this should be moved to Talos access adapter.
func (clusterAPI *ClusterAPI) GetRestConfig(ctx context.Context) (*rest.Config, error) {
	kubeconfig, err := clusterAPI.getKubeconfig(ctx)
	if err != nil {
		return nil, err
	}

	return clientcmd.BuildConfigFromKubeconfigGetter("", func() (*clientcmdapi.Config, error) {
		return clientcmd.LoadFromFile(kubeconfig.Path)
	})
}

// Install the ClusterAPI components and wait for them to be ready.
func (clusterAPI *ClusterAPI) Install(ctx context.Context) error {
	kubeconfig, err := clusterAPI.getKubeconfig(ctx)
	if err != nil {
		return err
	}

	options := client.InitOptions{
		Kubeconfig:              kubeconfig,
		CoreProvider:            "",
		BootstrapProviders:      clusterAPI.Options.BootstrapProviders,
		ControlPlaneProviders:   clusterAPI.Options.ControlPlaneProviders,
		InfrastructureProviders: clusterAPI.Options.InfrastructureProviders,
		TargetNamespace:         "",
		WatchingNamespace:       "",
		LogUsageInstructions:    false,
	}

	_, err = clusterAPI.clientset.CoreV1().Namespaces().Get(ctx, "sidero-system", metav1.GetOptions{})
	if err != nil {
		_, err = clusterAPI.client.Init(options)
		if err != nil {
			return err
		}
	}

	return clusterAPI.patch(ctx)
}

func (clusterAPI *ClusterAPI) patch(ctx context.Context) error {
	const (
		sideroNamespace         = "sidero-system"
		sideroMetadataServer    = "sidero-metadata-server"
		sideroControllerManager = "sidero-controller-manager"
	)

	// sidero-metadata-server
	deployment, err := clusterAPI.clientset.AppsV1().Deployments(sideroNamespace).Get(ctx, sideroMetadataServer, metav1.GetOptions{})
	if err != nil {
		return err
	}

	deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, "--port=9091")
	deployment.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
		{
			ContainerPort: 9091,
			Name:          "http",
		},
	}
	deployment.Spec.Template.Spec.HostNetwork = true
	deployment.Spec.Strategy.RollingUpdate = nil
	deployment.Spec.Strategy.Type = appsv1.RecreateDeploymentStrategyType

	_, err = clusterAPI.clientset.AppsV1().Deployments(sideroNamespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	// sidero-controller-manager
	deployment, err = clusterAPI.clientset.AppsV1().Deployments(sideroNamespace).Get(ctx, sideroControllerManager, metav1.GetOptions{})
	if err != nil {
		return err
	}

	deployment.Spec.Template.Spec.Containers[1].Args = append(deployment.Spec.Template.Spec.Containers[1].Args,
		fmt.Sprintf("--api-endpoint=%s", clusterAPI.bootstrapCluster.MasterIP()), "--metrics-addr=127.0.0.1:8080", "--enable-leader-election")
	deployment.Spec.Template.Spec.HostNetwork = true
	deployment.Spec.Strategy.RollingUpdate = nil
	deployment.Spec.Strategy.Type = appsv1.RecreateDeploymentStrategyType

	_, err = clusterAPI.clientset.AppsV1().Deployments(sideroNamespace).Update(ctx, deployment, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}
