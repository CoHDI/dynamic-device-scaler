# dynamic-device-scaler

Dynamic Device Scaler and Composable DRA Driver are components that enable dynamic device scaling (adding and removing) without OS reboots, based on requests from Pods. They bridge the gap between:
- k8s' [Dynamic Resource Allocation](https://kubernetes.io/docs/concepts/scheduling-eviction/dynamic-resource-allocation/) feature
- Composable Disaggregated Infrastructure

Composable Disaggregated Infrastructure is a new architecture that centrally manages devices such as GPUs in a pool, allowing for the flexible, software-defined configuration of Composed Baremetal, which combines servers and devices as needed. Each device and server is connected to a PCIe or CXL switch, and the control of this switch enables dynamic attachment and detachment of devices.
This technology enables efficient device utilization by sharing expensive devices across multiple nodes, providing only the necessary resources when needed.

![Composable Disaggregated Infrastructure](doc/cdi.png)

In k8s, to leverage Composable Disaggregated Infrastructure and achieve efficient device utilization, an intelligent component is required to scale devices up as needed based on workload demands, and scale them down when they are no longer required.
Dynamic Device Scaler is the solution. It works in conjunction with k8s' DRA feature and the scheduler to issue instructions for device scaling in response to device requests from Pods.

## Description

The following components work together to achieve dynamic scaling:


- **k8s Dynamic Resource Allocation**: Provides a mechanism for allocating hardware devices (e.g., GPUs) to Pods based on their requirements. 
 It defines available device listings (ResourceSlice), representing a pool of allocatable hardware resources with specific characteristics.
 Pods request resources through Device requests (ResourceClaim), specifying the desired type and quantity of hardware devices.
- **k8s Scheduler**: Schedules Pods. Considers DRA device requests and determines Pod placement. Implements the DRA feature and [KEP-5007](https://github.com/kubernetes/enhancements/tree/master/keps/sig-scheduling/5007-device-attach-before-pod-scheduled) to operate with composable devices.
- **vendor DRA plugin**: Provides a 3rd party DRA driver as a kubelet plugin, provided by device vendors. Outputs a list of devices attached to the node as a ResourceSlice. Prepares the devices for the Pod to operate.
- **Composable Resource Operator**: Receives device attach/detach instructions via CRs and instructs the Composable Disaggregated Infrastructure manager accordingly.
- **Composable Disaggregated Infrastructure manager**: Manages Composable Disaggregated Infrastructure.
- **Composable DRA Driver**: Publishes available node-free devices to Kubernetes.
- **Dynamic Device Scaler**: Instructs the CDI Operator (via CRs) to scale devices based on requests of Pods.

![How Dynamic Device Scaler Works](doc/dds1.png)

Dynamic scaling using Dynamic Device Scaler and Composable DRA Driver is achieved through the following process.
See also [KEP-5007](https://github.com/kubernetes/enhancements/tree/master/keps/sig-scheduling/5007-device-attach-before-pod-scheduled).

1. Composable DRA Driver initially outputs device information from the ResourcePool as a ResourceSlice. This ResourceSlice is assigned a special flag (BindingCondition).
For devices attached to the node, the device vendor's DRA driver outputs the ResourceSlice.
2. A Pod requests a device. The device request is written in a ResourceClaim.
3. The scheduler matches the ResourceSlice and ResourceClaim, and temporarily assigns a device in the ResourcePool to the Pod.
4. For ResourceSlices with a BindingCondition, the scheduler waits until the BindingCondition is met.
5. The DDS understands the above situation assigned by the scheduler and instructs the Composable Resource Operator to attach the device.
6. The Composable Resource Operator instructs the Composable Disaggregated Infrastructure manager to scale up.
7. The DDS checks the device attachment status, and if it recognizes that the device is attached, it informs the scheduler (reschedule instruction).

After this point, the Pod is scheduled in the same way as traditional DRA with devices attached to the node.

## Getting Started

### Prerequisites
- go version v1.23.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/dynamic-device-scaler:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/dynamic-device-scaler:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/dynamic-device-scaler:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/dynamic-device-scaler/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v1-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

