// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package setup

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	talosnet "github.com/talos-systems/net"
	"github.com/talos-systems/talos/pkg/cluster/check"
	clientconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/bundle"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/generate"
	"github.com/talos-systems/talos/pkg/provision"
	"github.com/talos-systems/talos/pkg/provision/access"
	"github.com/talos-systems/talos/pkg/provision/providers/qemu"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	"github.com/talos-systems/sfyra/pkg/config"
)

// BootstrapCluster sets up initial Talos cluster.
type BootstrapCluster struct {
	Options *config.Options

	provisioner provision.Provisioner
	cluster     provision.Cluster
	access      *access.Adapter

	masterIP net.IP

	stateDir   string
	configPath string
}

// NewBootstrapCluster creates BootstrapCluster.
func NewBootstrapCluster(ctx context.Context, options *config.Options) (*BootstrapCluster, error) {
	cluster := &BootstrapCluster{
		Options: options,
	}

	var err error
	cluster.provisioner, err = qemu.NewProvisioner(ctx)

	if err != nil {
		return nil, err
	}

	return cluster, nil
}

// Setup the bootstrap cluster.
func (cluster *BootstrapCluster) Setup(ctx context.Context) error {
	var err error

	cluster.configPath, err = clientconfig.GetDefaultPath()
	if err != nil {
		return err
	}

	defaultStateDir, err := clientconfig.GetTalosDirectory()
	if err != nil {
		return err
	}

	cluster.stateDir = filepath.Join(defaultStateDir, "clusters")

	fmt.Printf("bootstrap cluster state directory: %s, name: %s\n", cluster.stateDir, cluster.Options.BootstrapClusterName)

	if err = cluster.findExisting(ctx); err != nil {
		fmt.Printf("bootstrap cluster not found: %s, creating new one\n", err)

		err = cluster.create(ctx)
	}

	if err != nil {
		return err
	}

	checkCtx, checkCtxCancel := context.WithTimeout(ctx, 10*time.Minute)
	defer checkCtxCancel()

	if err = check.Wait(checkCtx, cluster.access, check.DefaultClusterChecks(), check.StderrReporter()); err != nil {
		return err
	}

	return cluster.untaint(ctx)
}

func (cluster *BootstrapCluster) findExisting(ctx context.Context) error {
	var err error

	cluster.cluster, err = cluster.provisioner.Reflect(ctx, cluster.Options.BootstrapClusterName, cluster.stateDir)
	if err != nil {
		return err
	}

	config, err := clientconfig.Open(cluster.configPath)
	if err != nil {
		return err
	}

	config.Context = cluster.Options.BootstrapClusterName

	cluster.access = access.NewAdapter(cluster.cluster, provision.WithTalosConfig(config))

	return nil
}

func (cluster *BootstrapCluster) create(ctx context.Context) error {
	_, cidr, err := net.ParseCIDR(cluster.Options.CIDR)
	if err != nil {
		return err
	}

	var gatewayIP net.IP

	gatewayIP, err = talosnet.NthIPInNetwork(cidr, 1)
	if err != nil {
		return err
	}

	ips := make([]net.IP, 1+cluster.Options.Nodes)

	for i := range ips {
		ips[i], err = talosnet.NthIPInNetwork(cidr, i+2)
		if err != nil {
			return err
		}
	}

	request := provision.ClusterRequest{
		Name: cluster.Options.BootstrapClusterName,

		Network: provision.NetworkRequest{
			Name:        cluster.Options.BootstrapClusterName,
			CIDR:        *cidr,
			GatewayAddr: gatewayIP,
			MTU:         1500,
			Nameservers: defaultNameservers,
			CNI: provision.CNIConfig{
				BinPath:  defaultCNIBinPath,
				ConfDir:  defaultCNIConfDir,
				CacheDir: defaultCNICacheDir,
			},
		},

		KernelPath:    cluster.Options.BootstrapTalosVmlinuz,
		InitramfsPath: cluster.Options.BootstrapTalosInitramfs,

		SelfExecutable: cluster.Options.TalosctlPath,
		StateDirectory: cluster.stateDir,
	}

	defaultInternalLB, _ := cluster.provisioner.GetLoadBalancers(request.Network)

	genOptions := cluster.provisioner.GenOptions(request.Network)

	for _, registryMirror := range cluster.Options.RegistryMirrors {
		parts := strings.SplitN(registryMirror, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("unexpected registry mirror format: %q", registryMirror)
		}

		genOptions = append(genOptions, generate.WithRegistryMirror(parts[0], parts[1]))
	}

	cluster.masterIP = ips[0]
	masterEndpoint := ips[0].String()

	configBundle, err := bundle.NewConfigBundle(bundle.WithInputOptions(
		&bundle.InputOptions{
			ClusterName: cluster.Options.BootstrapClusterName,
			Endpoint:    fmt.Sprintf("https://%s:6443", defaultInternalLB),
			GenOptions: append(
				genOptions,
				generate.WithEndpointList([]string{masterEndpoint}),
				generate.WithInstallImage(cluster.Options.BootstrapTalosInstaller),
			),
		}))
	if err != nil {
		return err
	}

	request.Nodes = append(request.Nodes,
		provision.NodeRequest{
			Name:     bootstrapMaster,
			IP:       ips[0],
			Memory:   cluster.Options.MemMB * 1024 * 1024,
			NanoCPUs: cluster.Options.CPUs * 1000 * 1000 * 1000,
			DiskSize: cluster.Options.DiskGB * 1024 * 1024 * 1024,
			Config:   configBundle.ControlPlane(),
		})

	for i := 0; i < cluster.Options.Nodes; i++ {
		request.Nodes = append(request.Nodes,
			provision.NodeRequest{
				Name:             fmt.Sprintf("pxe-%d", i),
				IP:               ips[i+1],
				Memory:           cluster.Options.MemMB * 1024 * 1024,
				NanoCPUs:         cluster.Options.CPUs * 1000 * 1000 * 1000,
				DiskSize:         cluster.Options.DiskGB * 1024 * 1024 * 1024,
				PXEBooted:        true,
				TFTPServer:       masterEndpoint,
				IPXEBootFilename: fmt.Sprintf("http://%s:8081/boot.ipxe", masterEndpoint),
			})
	}

	cluster.cluster, err = cluster.provisioner.Create(ctx, request, provision.WithBootlader(true), provision.WithTalosConfig(configBundle.TalosConfig()))
	if err != nil {
		return err
	}

	cluster.access = access.NewAdapter(cluster.cluster, provision.WithTalosConfig(configBundle.TalosConfig()))

	c, err := clientconfig.Open(cluster.configPath)
	if err != nil {
		return err
	}

	if c.Contexts == nil {
		c.Contexts = map[string]*clientconfig.Context{}
	}

	c.Contexts[cluster.Options.BootstrapClusterName] = configBundle.TalosConfig().Contexts[cluster.Options.BootstrapClusterName]

	c.Context = cluster.Options.BootstrapClusterName

	if err = c.Save(cluster.configPath); err != nil {
		return err
	}

	if err = cluster.access.Bootstrap(ctx, os.Stderr); err != nil {
		return err
	}

	return nil
}

func (cluster *BootstrapCluster) untaint(ctx context.Context) error {
	clientset, err := cluster.access.K8sClient(ctx)
	if err != nil {
		return err
	}

	n, err := clientset.CoreV1().Nodes().Get(ctx, bootstrapMaster, metav1.GetOptions{})
	if err != nil {
		return err
	}

	oldData, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("failed to marshal unmodified node %q into JSON: %w", n.Name, err)
	}

	n.Spec.Taints = []corev1.Taint{}

	newData, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("failed to marshal modified node %q into JSON: %w", n.Name, err)
	}

	patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, corev1.Node{})
	if err != nil {
		return fmt.Errorf("failed to create two way merge patch: %w", err)
	}

	if _, err := clientset.CoreV1().Nodes().Patch(ctx, n.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}); err != nil {
		return fmt.Errorf("error patching node %q: %w", n.Name, err)
	}

	return nil
}

// TearDown the bootstrap cluster.
func (cluster *BootstrapCluster) TearDown(ctx context.Context) error {
	if cluster.cluster != nil {
		if err := cluster.provisioner.Destroy(ctx, cluster.cluster); err != nil {
			return err
		}

		cluster.cluster = nil
	}

	return nil
}

// Access returns cluster access adapter.
func (cluster *BootstrapCluster) Access() *access.Adapter {
	return cluster.access
}

// MasterIP returns the IP of the master node.
func (cluster *BootstrapCluster) MasterIP() net.IP {
	return cluster.masterIP
}

// Nodes return information about PXE VMs.
func (cluster *BootstrapCluster) Nodes() []provision.NodeInfo {
	return cluster.cluster.Info().ExtraNodes
}
