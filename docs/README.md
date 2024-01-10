# Castle Lake (CSL): Exclusive Cache Allocation Technology (ExCAT)
The Exclusive Cache Allocation Technology (ExCAT) service is part of Castle Lake (CSL) and enables the usage of Intel's Cache Allocation Technology (CAT) for workloads that are orchestrated within a Kubernetes cluster. A user can request a cache buffer to be available exclusively for a workload. For that, an annotation is added to the Pod Spec with a key that determines the cache level, and the value specifying the size. The workload is then scheduled on a worker node that provides a matching cache buffer. The buffers have to be configured on the worker nodes before they can be used by the service. More details about CAT in general and how to use it in the context of the ExCAT service can be found in the following sections.

# Table of Content

<!-- TOC GFM -->

- [Intel® RDT Cache Allocation Technology (CAT)](#intel-rdt-cache-allocation-technology-cat)
  - [Usage via pseudo-file system `/sys/fs/resctrl`](#usage-via-pseudo-file-system-sysfsresctrl)
  - [Integration in Kubernetes Container Runtime](#integration-in-kubernetes-container-runtime)
    - [Configuration of resctrl classes](#configuration-of-resctrl-classes)
- [Prerequisites](#prerequisites)
  - [Supported HW](#supported-hw)
  - [Required SW](#required-sw)
  - [Configuration of exclusive cache buffers](#configuration-of-exclusive-cache-buffers)
- [Build](#build)
- [Installation](#installation)
  - [Make images available on the nodes](#make-images-available-on-the-nodes)
  - [Deployment using Helm](#deployment-using-helm)
- [Usage](#usage)
  - [ExCAT request in Pod/Deployment Spec](#excat-request-in-poddeployment-spec)
  - [Deploy workload](#deploy-workload)
- [Security considerations](#security-considerations)
- [Troubleshooting](#troubleshooting)
  - [Combination with TCC](#combination-with-tcc)
  - [CPU with Memory Bandwidth Allocation (MBA) support](#cpu-with-memory-bandwidth-allocation-mba-support)

<!-- /TOC -->

# Intel® RDT Cache Allocation Technology (CAT)
The Cache Allocation Technology (CAT) is part of the Intel® Resource Director Technology (RDT) feature set that provides a set of allocation capabilities with CAT being one of them. In the following, a brief overview of the CAT feature is given. More details about RDT and CAT can be found in the [Intel® 64 and IA-32 Architectures Software Developer’s Manual Chapter 17.18 and 17.19 (SDM volume 3)](https://www.intel.com/content/www/us/en/developer/articles/technical/intel-sdm.html).

Based on the CAT feature, the available cache can be separated into several sections. While the sections can be configured to overlap, they should be configured as non-overlapping sections in order to guarantee exclusive access in the context of ExCAT. Each of these portions of cache is thereby configured in the context of a class of service (CLOS) that is directly linked to it. Classes of service are referred to as classes within this document. A class thereby not only specifies the area of L2 and/or L3 cache, but also additional attributes like the logical CPUs associated with that class. CAT allows for assigning threads to one of the preconfigured classes and thereby to the portion of cache defined to be used for this class. All the configuration steps as well as the assignment of threads to classes is performed based on several registers. For accessing them, several tools and methods are available like

 * `pqos` and `rdtset`, both providing CLI tools that can be found [here](https://github.com/intel/intel-cmt-cat)
 * Linux RDT kernel driver that enables a pseudo-file system representing the RDT features at `/sys/fs/resctrl`

The usage based on the pseudo-file system at `/sys/fs/resctrl` is integrated into container runtime tools such as containerd, cri-o and run-c.

**Note:** To be able to use the Linux RDT kernel driver and the pseudo-file system that it exposes, the feature has to be enabled in the kernel config based on the `CONFIG_X86_CPU_RESCTRL` flag.

## Usage via pseudo-file system `/sys/fs/resctrl`
The Linux User Interface for Resource Control feature is described in detail in [the resctrl kernel doc](https://docs.kernel.org/arch/x86/resctrl.html). In the following, only the parts relevant for the ExCAT feature are briefly discussed. The resctrl kernel driver uses directories to describe the configured classes and files within each of these class directories that represent the current configuration. Such a configuration can be dynamically changed by writing into the respective files. 

The top level directory in `/sys/fs/resctrl` thereby is the default class `CLOS0` or `system/default` as it's sometimes called. All threads are assigned to this class per default when the system boots with the default class having access to the whole cache. Additional classes can then be created by creating new directories within `/sys/fs/resctrl`.

The top level `info` directory provides general information about the feature considering the underlying hardware capabilities. `info/L3/num_closids:16` e.g. means that the system supports up to 16 classes on the L3 cache. Each of these classes can be configured to access an area of L3 cache based on a bit mask with a maximum of e.g. `info/L3/cbm_mask=fffff`. Each capacity bit mask (CBM) has to use a contiguous block of 1s with the minimum number of bits given in `info/L3/min_cbm_bits`.

Each of the classes contain several files in the directory that represents the respective class. Together these files describe the configuration and utilization of this class. The most important files in the context of this project are

 * `schemata`: capacity bit mask that defines the amount and area of cache used for a certain level and cache ID. The latter is used instead of `socket` or `core` to define a set of logical CPUs sharing the cache.
 * `size`: size in bytes that result from the config in `schemata`.
 * `cpus`: bit mask of logical CPUs owned by the class.
 * `tasks`: list of process IDs of threads assigned to this class.

To be able to use the resctrl pseudo-file system, it has to be mounted first based on

```bash
mount -t resctrl resctrl /sys/fs/resctrl
```

If more RDT features besides CAT should be used, there are additional options that could be added to the mount command as explained in the [kernel docs](https://docs.kernel.org/arch/x86/resctrl.html).

## Integration in Kubernetes Container Runtime 
Intel's RDT is part of the [OCI specs](https://github.com/opencontainers/runtime-spec/blob/main/config-linux.md#intelrdt) and the utilization of RDT CAT is integrated into the following container runtimes based on the abstraction using the pseudo-file system at `/sys/fs/resctrl`:

 * [containerd](https://github.com/containerd/containerd) since [version 1.6.0](https://github.com/containerd/containerd/releases/tag/v1.6.0)
 * [cri-o](https://github.com/cri-o/cri-o) since [version 1.22.0](https://github.com/cri-o/cri-o/releases/tag/v1.22.0)
 * [runc](https://github.com/opencontainers/runc) since [version 1.1.9](https://github.com/opencontainers/runc/releases/tag/v1.1.9)

This enables automated

  * configuration of classes within the pseudo-file system based on a yaml spec file
  * assignment of pods and containers to classes by means of added annotations

**Note:** Intel RDT does not work when using docker. If you want to use ExCAT, cri-o or containerd has to be used as the Kubernetes runtime.

### Configuration of resctrl classes
In the following the configuration based on containerd is explained. For details about using RDT with cri-o refer to [cri-o docs](https://github.com/cri-o/cri-o/blob/main/docs/crio.8.md).

For containerd, RDT support is enabled as a plugin by providing the following section to containerd's config file that usually resides at `/etc/containerd/config.toml`:

```bash
$ cat /etc/containerd/config.toml
version = 2

[plugins]
  [plugins."io.containerd.service.v1.tasks-service"]
    rdt_config_file = "/etc/rdt-config.yaml"
```

The provided path for `rdt_config_file` thereby points to a yaml file that contains the desired config of CAT classes. The default location is `/etc/rdt-config.yaml`. The configuration in that file is then automatically transferred into the matching file system-structure in `/sys/fs/resctrl` including the right files and it's content describing the classes cache-level, sizes and so on. This is done when containerd starts. If you want containerd to reconfigure based on some changes in the yaml file, you have to e.g. restart the containerd service. For the configuration to work, one has to write the config file (e.g. `/etc/rdt-config.yaml`) according to [this format description](https://github.com/intel/goresctrl/blob/main/doc/rdt.md#configuration-format).

For the ExCAT service, only some of the available config options are relevant and explained based on the following example.

```yaml
# Common options
options:
  l2:
    # Set to false if L2 CAT must be available (Default is true).
    optional: true
  l3:
    # Set to false if L3 CAT must be available (Default is true).
    optional: true
  mb:
    # Set to false if MBA must be available (Default is true).
    optional: true
# Partition definitions
partitions:
  pdef:
    l2Allocation: "50%"
    l3Allocation: "50%"
    classes:
      system/default:
        l2schema: "100%"
        l3schema: "100%"
  p0:
    l2Allocation: "25%"
    l3Allocation: "25%"
    classes:
      c0:
        l2schema: "100%"
        l3schema: "100%"
  p1:
    l2Allocation: "25%"
    l3Allocation: "25%"
    classes:
      c1:
        l2schema: "100%"
        l3schema: "100%"
```

In the `options` section, it's possible to specify whether the availability of L2 and L3 cache is a hard requirement. The `mb` options is only relevant in the context of the Code and Data Prioritization (CDP) feature that's not utilized by ExCAT.

The term `partitions` is used here to distinguish from regular `classes` in that partitions do not overlap. In this example, we're defining 3 partitions:

 1. `pdef`: occupies 50% of the whole cache, no matter if it's L2 or L3 cache. This partition is required and keeps the default class `system/default` that is represented by the root directory in `/sys/fs/resctrl` and that utilizes 100% of the partition's space, so 50% of the complete cache space in this example.
 2. `p0`: occupies 25% of the complete cache and keeps a class `c0` that utilizes 100% of the partition's space.
 3. `p1`: occupies 25% of the complete cache and keeps a class `c1` that utilizes 100% of the partition's space.

If we would define several classes within the same partition, these classes could overlap. By defining partitions with just one class that occupies 100% of the partition's space, it is ensured that all defined classes do not overlap. This enables for exclusive cache. It is important to know, that containerd executes the configuration so that all resctrl [cache_ids](https://docs.kernel.org/arch/x86/resctrl.html#cache-ids) are configured in exactly the same way. Consider the following example with 2 classes being defined

 1. class `C1` occupying 1/3 of L2 cache
 2. class `C2` occupying 1/2 of L3 cache

The according yaml file would then look like so

```yaml
# Common options
options:
  l2:
    # Set to false if L2 CAT must be available (Default is true).
    optional: true
  l3:
    # Set to false if L3 CAT must be available (Default is true).
    optional: true
  mb:
    # Set to false if MBA must be available (Default is true).
    optional: true
# Partition definitions
partitions:
  pdef:
    l2Allocation: "66%"
    l3Allocation: "50%"
    classes:
      system/default:
        l2schema: "100%"
        l3schema: "100%"
  p0:
    l2Allocation: "34%"
    classes:
      C0:
        l2schema: "100%"
  p1:
    l3Allocation: "50%"
    classes:
      C1:
        l2schema: "100%"
        l3schema: "100%"
```

To ensure the two classes are exclusive, we limit the default class `system/default` to only use the remaining 2/3 of L2 cache and 50% of L3 cache. Let's assume our node has 2 sockets with 4 CPUs each. 2 CPUs share one L2 cache while all CPUs on the socket share one L3 cache. Containerd would then configure the classes so that all cache_ids would be configured in exactly the same way. In other words, all available L2 caches are configured the same and all L3 caches are configured the same, respectively. This is depicted in the following diagram.

<img src="assets/resctrl_cacheID_config.png" alt="configuration of cache_ids" style="width:703px;height:188px;">

This way, it doesn't really matter on which specific CPU a workload happens to run, since the cache it can access is always configured in the same way.

**Note:** Using the CSL TCC service on the same node results in different sized caches for the available cache_ids. Since the CAT buffer size is configured based on specifying a percentage of the whole cache, the resulting buffers are also differently sized. In such a case, only the smalles buffer size is advertized within the cluster and thus some cache space is wasted for the bigger buffers.

**Note:** If more determinism is required, a pod can be configured to be assigned to a Quality of Service (QoS) class of Guaranteed so that it gets one or more exclusive cpu cores and a dedicated amount of memory. More details on this can be found [in the K8s documentation](https://kubernetes.io/docs/tasks/configure-pod-container/quality-service-pod/). For this to work, the kubelet on the node where the pod is to run has to be started with the static CPU Manager Policy as explained [here](https://kubernetes.io/docs/tasks/administer-cluster/cpu-management-policies/#cpu-management-policies).

# Prerequisites
## Supported HW
CAT support can be checked based on the CPU info flags. An easy way to do that is to use

```bash
lscpu | grep cat_l
```

with the presence of `cat_l2` and `cat_l3` representing the support of CAT for level 2 and level 3 cache, respectively. Another way is to use the command `cpuid`. 

**Note:** Currently processors like the Scalable Xeons that support both, level 2 and level 3 cache, are not supported by the ExCAT service.

## Required SW
As explained in section [Integration in Kubernetes container runtime](#integration-in-kubernetes-container-runtime), the following container runtimes have to be used for ExCAT to work:

* [runc](https://github.com/opencontainers/runc) version 1.1.9 or later *and*
* *either* [containerd](https://github.com/containerd/containerd) version 1.6.0 or later
* *or* [cri-o](https://github.com/cri-o/cri-o) version 1.22.0 or later

**Note:** Intel RDT does not work when using docker. If you want to use ExCAT, cri-o or containerd has to be used as the Kubernetes runtime.

## Configuration of exclusive cache buffers
On each worker node that should provide cache buffers to be used when pods request exclusive cache, the cache buffers first have to be configured. The configuration is done based on a yaml file as explained in section [Configuration of exclusive cache buffers](#configuration-of-resctrl-classes). So far, only one cache buffer size is supported per node. That means that ExCAT will determine the smallest configured buffer on a node and advertise this size for all configured buffers on this node. It thus makes sense to use one size for all buffers on one node to not waste cache space. Also note that the resctrl pseudo-file system has to be mounted as explained in [Usage via pseudo-file system `/sys/fs/resctrl`](#usage-via-pseudo-file-system-sysfsresctrl).

# Build
In case source code is part of the release package, the project can be built based on the provided `Makefile`. A list with all possible targets including a short description can be obtained with `make help`.

```bash
$ make help
Usage:

  setup               1st time set up of the dev environment
  build               cleans up and builds the go code
  buildincnt          build inside golang container
  test                run unit tests
  clean               cleans the image and binary
  image               cleans, builds the go code and builds an image with podman
  image2cluster       builds the image and adds it to the current host. Can be used if the the cri is containerd and the host is part of the cluster. Requires `ctr`.
  helm                deploys the helm chart to a cluster. Requires to run `image` first
  unhelm              uninstalls the helm release installed before with `make helm`
  package             create release tar.gz package
  test-setup          creates a minikube cluster with 2 VM-nodes using kvm2
  test-clean-deploy   removes excat images from the minikube test-cluster
  test-image-deploy   loads excat images into the minikube test-cluster
  test-helm           installs the excat helm chart into the minikube test-cluster
  test-destroy        deletes the minikube test-cluster
  help                prints this help message
```

The main components of a release package are

  * images (`csl-excat-admission.tar` and `csl-excat-deviceplugin.tar`)
  * helm chart (for deployment of ExCAT within a cluster)
  * docs (README and openshift specific doc)
  * licenses of dependencies

The components that can be build from source are the two images and the html documentation. For this to work, the following dependencies have to be installed:

  * make
  * [golang](https://go.dev/doc/install) 1.18
  * [podman](https://podman.io/getting-started/installation)
  * [pandoc](https://pandoc.org/installing.html)

The `make setup` partly automates this, but also installs a pre-commit based linter.

To build the images do

```bash
make image
```

A full release package can be build with

```bash
make package
```

that will result in `./build/csl-excat-<TAG>.tar.gz` with `TAG` being a tag that can be set in the top of the `Makefile`. 

# Installation
## Make images available on the nodes
The following images are available as tar-balls in `./images`:

 1. `csl-excat-admission.tar`
 2. `csl-excat-deviceplugin.tar`

To make the images available for usage in the cluster, they can be either pushed to a registry or they can be copied to the nodes and imported using e.g. containerd's cli tool `ctr`. If a registry is not available, make sure to copy `csl-excat-admission.tar` to the control-plane nodes and `csl-excat-deviceplugin.tar` to all worker nodes. The import can then be done e.g. by

```bash
sudo ctr -n k8s.io images import <image.tar>
```

with `image.tar` being the image to import

## Deployment using Helm
To deploy the ExCAT service to the cluster, a helm chart is used that can be found in `./deployments/helm`. Before using it, the file `./deployments/helm/values.yaml` has to be updated:

 * Image repository and tag for both images: `admission.image.repository`/`admission.image.tag` and `deviceplugin.image.repository`/`deviceplugin.image.tag`.
 * In case a cert-manager is to be used, adapt the values of the `admission.tlsSecret` section for the following keys:

   * `admission.tlsSecret.certSource` with the value `cert-manager`.
   * `admission.tlsSecret.name` with the name created by the cert-manager.
   * `admission.tlsSecret.certmanagerAnnotations` with the required annotation for mutating webhook configuration.

 * The admission controller pod will by default be deployed on control plane nodes and device plugin pods on nodes with label: `excat=yes`. This can be changed using `nodeSelector` fields in the values file. To label the node where excat is configured use the command below:

**NOTE:** For Kubernetes 1.24 and above, `admission.nodeSelector.node-role.kubernetes.io/master` has to be changed to `admission.nodeSelector.node-role.kubernetes.io/control-plane` and `admission.tolerations.key.node-role.kubernetes.io/master` to `admission.tolerations.key.node-role.kubernetes.io/control-plane` to satisfy the name change (see [here](https://kubernetes.io/blog/2022/04/07/upcoming-changes-in-kubernetes-1-24/#api-removals-deprecations-and-other-changes-for-kubernetes-1-24) for more details).


```bash
kubectl label node <nodename> excat=yes
```

The deployment is then done based on the following commands:

Without a cert-manager:

```bash
APP=csl-excat
NAMESPACE=excat
VALIDITY=365
cd deployments/helm/ && \
./gencerts.sh certs ${APP}-admission ${NAMESPACE} ${VALIDITY} && \
helm install ${APP} --create-namespace -n ${NAMESPACE} .
```
with `VALIDITY` being the duration in days that the admission controler's certificate will be valid for. If the `VALIDITY` argument is not passed to `gencerts.sh`, the certificate will last for 365 days.

With a cert-manager:

```bash
APP=csl-excat
NAMESPACE=excat
cd deployments/helm/ && \
helm install ${APP} --create-namespace -n ${NAMESPACE} .
```

**NOTE**: cert-manager values can be overwritten using the `--set` parameter in the helm install command in the following way:

```bash
APP=csl-excat
NAMESPACE=excat
cd deployments/helm/ && \
helm install ${APP} --create-namespace -n ${NAMESPACE} . \
	--set admission.tlsSecret.create=false \
	--set admission.tlsSecret.name=excat-tls-secret \
	--set admission.tlsSecret.certSource=cert-manager \
```

# Usage
## ExCAT request in Pod/Deployment Spec
The service has to be enabled by adding `excat: "yes"` as a label like so

```bash
metadata:
  labels:
    excat: "yes"
```
To request an exclusive cache buffer of a certain size for a specific cache level, an annotation has to be added to the Pod or Deployment Spec like so

```yaml
annotations:
  intel.com/excat-l<cache_level>: "<size_in_kib>"
```

with `<cache_level>` being `2` or `3` and <size_in_kib> being the requested size in Kibi-Byte. An example Pod Spec file `myExample.yaml` is given in the following:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: example
  labels:
    app: example
    excat: "yes"
  annotations:
    intel.com/excat-l3: "200"
spec:
  containers:
  - name: example
    image: docker.io/library/example:latest
    imagePullPolicy: IfNotPresent
```

## Deploy workload
Then just deploy the Pod or Deployment as usual with

```bash
kubectl apply -f myExample.yaml
```
# Security considerations

The ExCAT service security is the same as of the cluster where it is deployed. Make sure to follow Kubernetes security and best practices guidelines. For more information, see the [Kubernetes documentation](https://kubernetes.io/docs/tasks/administer-cluster/securing-a-cluster/).
If the ExCAT admission controller is being installed using a self-signed certificate (no cert-manager), the TLS private key `excatadmission-tls.key` and
certificate pairs are generated under `deployments/helm/certs/` using `./gencerts.sh` script. Make sure all generated files permission are read only by the owner.
```bash
chmod go-rwx deployments/helm/certs/*
```

# Troubleshooting
## Combination with TCC
TCC and ExCAT do not work together. If TCC is used, it is recommended to disable CAT by means of `rdt=!l3cat,!l2cat` within the kernel command-line. In contrast to that, make sure that the CAT feature is not disabled explicitly within the kernel command-line if it is to be used based on e.g. ExCAT.
## CPU with Memory Bandwidth Allocation (MBA) support
In case the CPU not only supports CAT, but also [MBA](https://docs.kernel.org/arch/x86/resctrl.html?highlight=intel+resource#memory-bandwidth-allocation-and-monitoring) (check with `cat /proc/cpuinfo | grep mba`), the version of containerd or crio is important. If containerd with a version less then [1.7.0](https://github.com/containerd/containerd/releases/tag/v1.7.0) or crio with a version less then [1.26.0](https://github.com/cri-o/cri-o/releases/tag/v1.26.0) is used, the `rdt_config_file` has to be adapted to explicitly specify memory bandwidth for each class like so:

```yaml
# Common options
options:
  l2:
    # Set to false if L2 CAT must be available (Default is true).
    optional: true
  l3:
    # Set to false if L3 CAT must be available (Default is true).
    optional: true
  mb:
    # Set to false if MBA must be available (Default is true).
    optional: true
# Partition definitions
partitions:
  pdef:
    l2Allocation: "50%"
    l3Allocation: "50%"
    mbAllocation: ["100%"]
    classes:
      system/default:
        l2schema: "100%"
        l3schema: "100%"
  p0:
    l2Allocation: "25%"
    l3Allocation: "25%"
    mbAllocation: ["50%"]
    classes:
      c0:
        l2schema: "100%"
        l3schema: "100%"
  p1:
    l2Allocation: "25%"
    l3Allocation: "25%"
    mbAllocation: ["50%"]
    classes:
      c1:
        l2schema: "100%"
        l3schema: "100%"
```
