// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package setup provisions bootstrap cluster, installs cluster API, etc.
package setup

import "net"

var (
	defaultNameservers = []net.IP{net.ParseIP("8.8.8.8"), net.ParseIP("1.1.1.1")}
	defaultCNIBinPath  = []string{"/opt/cni/bin"}
)

const (
	defaultCNIConfDir  = "/etc/cni/conf.d"
	defaultCNICacheDir = "/var/lib/cni"

	bootstrapMaster = "bootstrap-master"
)
