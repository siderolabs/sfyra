// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"testing"

	"github.com/talos-systems/talos/pkg/cli"

	"github.com/talos-systems/sfyra/pkg/config"
	"github.com/talos-systems/sfyra/pkg/setup"
	"github.com/talos-systems/sfyra/pkg/tests"
)

func main() {
	options := config.DefaultOptions()

	flag.BoolVar(&options.SkipTeardown, "skip-teardown", options.SkipTeardown, "skip tearing down cluster")
	flag.StringVar(&options.BootstrapClusterName, "bootstrap-cluster-name", options.BootstrapClusterName, "bootstrap cluster name")
	flag.StringVar(&options.BootstrapTalosVmlinuz, "bootstrap-vmlinuz", options.BootstrapTalosVmlinuz, "Talos kernel image for bootstrap cluster")
	flag.StringVar(&options.BootstrapTalosInitramfs, "bootstrap-initramfs", options.BootstrapTalosInitramfs, "Talos initramfs image for bootstrap cluster")
	flag.StringVar(&options.BootstrapTalosInstaller, "bootstrap-installer", options.BootstrapTalosInstaller, "Talos install image for bootstrap cluster")
	flag.StringVar(&options.CIDR, "cidr", options.CIDR, "network CIDR")
	flag.IntVar(&options.Nodes, "nodes", options.Nodes, "number of PXE nodes to create")
	flag.StringVar(&options.TalosctlPath, "talosctl-path", options.TalosctlPath, "path to the talosctl (for qemu provisioner)")
	flag.Var(&options.RegistryMirrors, "registry-mirrors", "registry mirrors to use")
	flag.StringVar(&options.TalosKernelURL, "talos-kernel-url", options.TalosKernelURL, "Talos kernel image URL for Cluster API Environment")
	flag.StringVar(&options.TalosInitrdURL, "talos-initrd-url", options.TalosInitrdURL, "Talos initramfs image URL for Cluster API Environment")

	testing.Init()

	flag.Parse()

	err := cli.WithContext(context.Background(), func(ctx context.Context) error {
		bootstrapCluster, err := setup.NewBootstrapCluster(ctx, &options)
		if err != nil {
			return err
		}

		if !options.SkipTeardown {
			defer bootstrapCluster.TearDown(ctx) //nolint: errcheck
		}

		if err = bootstrapCluster.Setup(ctx); err != nil {
			return err
		}

		clusterAPI, err := setup.NewClusterAPI(ctx, &options, bootstrapCluster)
		if err != nil {
			return err
		}

		if err = clusterAPI.Install(ctx); err != nil {
			return err
		}

		if ok := tests.Run(ctx, &options, bootstrapCluster, clusterAPI); !ok {
			return fmt.Errorf("test failure")
		}

		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}
