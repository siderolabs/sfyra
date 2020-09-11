// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"

	cabpt "github.com/talos-systems/cluster-api-bootstrap-provider-talos/api/v1alpha3"
	cacpt "github.com/talos-systems/cluster-api-control-plane-provider-talos/api/v1alpha3"
	sidero "github.com/talos-systems/sidero/app/cluster-api-provider-sidero/api/v1alpha3"
	metal "github.com/talos-systems/sidero/app/metal-controller-manager/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/cmd/clusterctl/client"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/talos-systems/sfyra/pkg/config"
)

// ClusterAPI installs and manages cluster API installation.
type ClusterAPI struct {
	Options *config.Options

	bootstrapCluster *BootstrapCluster

	kubeconfig    client.Kubeconfig
	client        client.Client
	clientset     *kubernetes.Clientset
	runtimeClient runtimeclient.Client
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

// GetKubeconfig returns kubeconfig in clusterctl expected format.
func (clusterAPI *ClusterAPI) GetKubeconfig(ctx context.Context) (client.Kubeconfig, error) {
	if clusterAPI.kubeconfig.Path != "" {
		return clusterAPI.kubeconfig, nil
	}

	kubeconfigBytes, err := clusterAPI.bootstrapCluster.Access().Kubeconfig(ctx)
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
func (clusterAPI *ClusterAPI) GetRestConfig(ctx context.Context) (*rest.Config, error) {
	return clusterAPI.bootstrapCluster.Access().K8sRestConfig(ctx)
}

// GetClusterAPIClient client returns instance of cluster API client.
func (clusterAPI *ClusterAPI) GetClusterAPIClient() client.Client {
	return clusterAPI.client
}

// GetMetalClient returns k8s client stuffed with CAPI CRDs.
func (clusterAPI *ClusterAPI) GetMetalClient(ctx context.Context) (runtimeclient.Client, error) {
	if clusterAPI.runtimeClient != nil {
		return clusterAPI.runtimeClient, nil
	}

	config, err := clusterAPI.GetRestConfig(ctx)
	if err != nil {
		return nil, err
	}

	scheme := runtime.NewScheme()

	if err = v1alpha3.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if err = cacpt.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if err = cabpt.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if err = sidero.AddToScheme(scheme); err != nil {
		return nil, err
	}

	if err = metal.AddToScheme(scheme); err != nil {
		return nil, err
	}

	clusterAPI.runtimeClient, err = runtimeclient.New(config, runtimeclient.Options{Scheme: scheme})

	return clusterAPI.runtimeClient, err
}

// Install the ClusterAPI components and wait for them to be ready.
func (clusterAPI *ClusterAPI) Install(ctx context.Context) error {
	kubeconfig, err := clusterAPI.GetKubeconfig(ctx)
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

	oldDeployment, err := json.Marshal(deployment)
	if err != nil {
		return err
	}

	argsPatched := false

	for _, arg := range deployment.Spec.Template.Spec.Containers[0].Args {
		if arg == "--port=9091" {
			argsPatched = true
		}
	}

	if !argsPatched {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, "--port=9091")
	}

	deployment.Spec.Template.Spec.Containers[0].Ports = []corev1.ContainerPort{
		{
			ContainerPort: 9091,
			HostPort:      9091,
			Name:          "http",
			Protocol:      corev1.ProtocolTCP,
		},
	}
	deployment.Spec.Template.Spec.HostNetwork = true
	deployment.Spec.Strategy.RollingUpdate = nil
	deployment.Spec.Strategy.Type = appsv1.RecreateDeploymentStrategyType

	newDeployment, err := json.Marshal(deployment)
	if err != nil {
		return err
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldDeployment, newDeployment, appsv1.Deployment{})
	if err != nil {
		return fmt.Errorf("failed to create two way merge patch: %w", err)
	}

	_, err = clusterAPI.clientset.AppsV1().Deployments(sideroNamespace).Patch(ctx, deployment.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{
		FieldManager: "sfyra",
	})
	if err != nil {
		return err
	}

	// sidero-controller-manager
	deployment, err = clusterAPI.clientset.AppsV1().Deployments(sideroNamespace).Get(ctx, sideroControllerManager, metav1.GetOptions{})
	if err != nil {
		return err
	}

	oldDeployment, err = json.Marshal(deployment)
	if err != nil {
		return err
	}

	argsPatched = false

	for _, arg := range deployment.Spec.Template.Spec.Containers[1].Args {
		if arg == "--metrics-addr=127.0.0.1:8080" {
			argsPatched = true
		}
	}

	if !argsPatched {
		deployment.Spec.Template.Spec.Containers[1].Args = append(deployment.Spec.Template.Spec.Containers[1].Args,
			fmt.Sprintf("--api-endpoint=%s", clusterAPI.bootstrapCluster.MasterIP()), "--metrics-addr=127.0.0.1:8080", "--enable-leader-election")
	}

	deployment.Spec.Template.Spec.HostNetwork = true
	deployment.Spec.Strategy.RollingUpdate = nil
	deployment.Spec.Strategy.Type = appsv1.RecreateDeploymentStrategyType

	newDeployment, err = json.Marshal(deployment)
	if err != nil {
		return err
	}

	patchBytes, err = strategicpatch.CreateTwoWayMergePatch(oldDeployment, newDeployment, appsv1.Deployment{})
	if err != nil {
		return fmt.Errorf("failed to create two way merge patch: %w", err)
	}

	_, err = clusterAPI.clientset.AppsV1().Deployments(sideroNamespace).Patch(ctx, deployment.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{
		FieldManager: "sfyra",
	})
	if err != nil {
		return err
	}

	return nil
}
