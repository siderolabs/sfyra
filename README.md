# sfyra

Integration test for Sidero/Arges.

## Running

Build the test binary:

    make integration-test

Run the test:

    make run-integration-test

Registry mirrors could be dropped if not being used.
Test uses CIDR `172.24.0.0/24` by default.

Sequence of steps:

* build initial bootstrap Talos cluster of one node
* install Cluster API, Sidero and Talos providers
* run the unit-tests

With `-skip-teardown` flag test leaves the bootstrap cluster running so that next iteration of the test
can be run without waiting for the boostrap actions to be finished.
