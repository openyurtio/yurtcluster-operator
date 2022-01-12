# YurtCluster Operator Architecture

The YurtCluster Operator consists of two core components: `yurtcluster-operator-manager` and `yurtcluster-operator-agent`.

- `yurtcluster-operator-manager` is a global coordinator component that is responsible for deploying/configuring cluster edge components and checking/updating the status of `YurtCluster` CR.

- `yurtcluster-operator-agent` focus on the execution of node conversion, it runs as a `DaemonSet`, and each Pod only deal with the convert/revert tasks related to the Node it is running on.
