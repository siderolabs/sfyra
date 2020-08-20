# sfyra

Integration test for Sidero/Arges.

## Running

Build the test binary:

    make integration-test

Test depends on several assets generated with Talos, so for now it's easier
to launch it in the source checkout of Talos:

    (cd ../talos/; sudo -E ../sfyra/_out/integration-test -skip-teardown -registry-mirrors docker.io=http://172.24.0.1:5000,k8s.gcr.io=http://172.24.0.1:5001,quay.io=http://172.24.0.1:5002,gcr.io=http://172.24.0.1:5003 -nodes 4 -test.v)

Registry mirrors could be dropped if not being used.
Test uses CIDR `172.24.0.0/24` by default.

Sequence of steps:

* build initial bootstrap Talos cluster of one node
* install Cluster API, Sidero and Talos providers
* run the unit-tests

With `-skip-teardown` flag test leaves the bootstrap cluster running so that next iteration of the test
can be run without waiting for the boostrap actions to be finished.
