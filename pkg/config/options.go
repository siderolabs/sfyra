// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

// Options control the sidero testing.
type Options struct {
	SkipTeardown bool

	BootstrapClusterName    string
	BootstrapTalosVmlinuz   string
	BootstrapTalosInitramfs string
	BootstrapTalosInstaller string

	TalosKernelURL string
	TalosInitrdURL string

	BootstrapProviders      []string
	InfrastructureProviders []string
	ControlPlaneProviders   []string

	RegistryMirrors stringSlice

	CIDR string

	Nodes int

	MemMB  int64
	CPUs   int64
	DiskGB int64

	TalosctlPath string
}

// DefaultOptions returns default settings.
func DefaultOptions() Options {
	return Options{
		BootstrapClusterName:    "sfyra",
		BootstrapTalosVmlinuz:   "_out/vmlinuz",
		BootstrapTalosInitramfs: "_out/initramfs.xz",
		BootstrapTalosInstaller: "docker.io/autonomy/installer:v0.7.0-alpha.1",

		TalosKernelURL: "https://github.com/talos-systems/talos/releases/download/v0.7.0-alpha.1/vmlinuz",
		TalosInitrdURL: "https://github.com/talos-systems/talos/releases/download/v0.7.0-alpha.1/initramfs.xz",

		BootstrapProviders:      []string{"talos"},
		InfrastructureProviders: []string{"sidero"},
		ControlPlaneProviders:   []string{"talos"},

		CIDR: "172.24.0.0/24",

		Nodes: 4,

		MemMB:  2048,
		CPUs:   2,
		DiskGB: 4,

		TalosctlPath: "_out/talosctl-linux-amd64",
	}
}
