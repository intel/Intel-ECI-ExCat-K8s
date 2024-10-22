# Table of Content

<!-- TOC -->

- [Openshift Configuration](#openshift-configuration)
  - [Introduction](#introduction)
  - [Node configuration](#node-configuration)
  - [SCC configuration](#scc-configuration)
  - [Images](#images)

<!-- /TOC -->

# Openshift Configuration

## Introduction

For the moment the ExCAT service does not provide an operator that automates the installation on an Openshift cluster. Thus, a few manual steps have to be done as explained in the following.

## Node configuration

Required kernel configs for excat are only supported on openshift realtime kernel config.
Performance Addon Operator need to be used to enable that.
<https://docs.openshift.com/container-platform/4.10/scalability_and_performance/cnf-performance-addon-operator-for-low-latency-nodes.html>

In addition resctrl sysfs need to be mounted, crio and rdt need to be configured. This can be done manually on the node (not recommended) or using the MCO (Machine Config Operator). For
more details about the MCO see <https://docs.openshift.com/container-platform/4.10/post_installation_configuration/machine-configuration-tasks.html#understanding-the-machine-config-operator>. This document also provides all cli commands related to machine configs. The most important ones are given below.

Machine configs are applied based on
```bash
oc create -f myMachineConfig.yaml
```
with `myMachineConfig.yaml` representing below files. One can then check the current status of the applied machine config with
```bash
oc get mcp worker
```
that shows the status of the worker machine config pool, so machine configs applied to worker nodes. Once the status `UPDATED` switches to `true`, all worker nodes have been rebooted and the machine config is applied.

Create the following machine configs for resctrl mount, crio and rdt config:

- crio configuration to enable RDT and allow the required annotations:

  ```yaml
  #[crio.runtime]
  #rdt_config_file = "/etc/rdt-config.yaml"
  #
  #[crio.runtime.runtimes.runc]
  #runtime_path = ""
  #runtime_type = "oci"
  #runtime_root = "/run/runc"
  #allowed_annotations = ["io.kubernetes.cri.rdt-class"]
  apiVersion: machineconfiguration.openshift.io/v1
  kind: MachineConfig
  metadata:
    labels:
      machineconfiguration.openshift.io/role: worker-rt
    name: 90-rdt-crio-conf
  spec:
    config:
      ignition:
        version: 3.2.0
      storage:
        files:
        - contents:
            source: data:text/plain;charset=utf-8;base64,W2NyaW8ucnVudGltZV0KcmR0X2NvbmZpZ19maWxlID0gIi9ldGMvcmR0LWNvbmZpZy55YW1sIgoKW2NyaW8ucnVudGltZS5ydW50aW1lcy5ydW5jXQpydW50aW1lX3BhdGggPSAiIgpydW50aW1lX3R5cGUgPSAib2NpIgpydW50aW1lX3Jvb3QgPSAiL3J1bi9ydW5jIgphbGxvd2VkX2Fubm90YXRpb25zID0gWyJpby5rdWJlcm5ldGVzLmNyaS5yZHQtY2xhc3MiXQo=
          mode: 420
          overwrite: true
          path: /etc/crio/crio.conf.d/99-setrdt.conf
  
  ```

- Add resctrl sysfs mount as a dependency of crio:

  ```yaml
  #cat << EOF | base64
  #[Unit]
  #Requires=sys-fs-resctrl.mount
  #
  #EOF
  
  
  apiVersion: machineconfiguration.openshift.io/v1
  kind: MachineConfig
  metadata:
    labels:
      machineconfiguration.openshift.io/role: worker-rt
    name: 90-resctlr-mount-config
  spec:
    config:
      ignition:
        version: 3.2.0
      storage:
        files:
        - contents:
            source: data:text/plain;charset=utf-8;base64,W1VuaXRdClJlcXVpcmVzPXN5cy1mcy1yZXNjdHJsLm1vdW50Cgo=
          mode: 420
          overwrite: true
          path: /etc/systemd/system/crio.service.d/10-resctrl-mount.conf
  ```

- RDT config file:

  ```yaml
  apiVersion: machineconfiguration.openshift.io/v1
  kind: MachineConfig
  metadata:
    labels:
      machineconfiguration.openshift.io/role: worker-rt
    name: 90-rdt-config
  spec:
    config:
      ignition:
        version: 3.2.0
      storage:
        files:
        - contents:
        source: data:text/plain;charset=utf-8;base64,IyBDb21tb24gb3B0aW9ucwpvcHRpb25zOgogIGwyOgogICAgb3B0aW9uYWw6IHRydWUKICBsMzoKICAgIG9wdGlvbmFsOiB0cnVlCiAgbWI6CiAgICBvcHRpb25hbDogdHJ1ZQojIFBhcnRpdGlvbiBkZWZpbml0aW9ucwpwYXJ0aXRpb25zOgogIHAxOgogICAgbDJBbGxvY2F0aW9uOiAiMHhmZmZmZiIKICAgIGwzQWxsb2NhdGlvbjogIjB4ZmZmIgogICAgY2xhc3NlczoKICAgICAgc3lzdGVtL2RlZmF1bHQ6CiAgICAgICAgbDJBbGxvY2F0aW9uOiAiMHhmZmZmYyIKICAgICAgICBsM0FsbG9jYXRpb246ICIweGZmMCIKICAgICAgQ09TMToKICAgICAgICBsMkFsbG9jYXRpb246ICIweDAwMDAzIgogICAgICAgIGwzQWxsb2NhdGlvbjogIjB4ZmYwIgogICAgICBDT1MyOgogICAgICAgIGwyQWxsb2NhdGlvbjogIjB4ZmZmZmMiCiAgICAgICAgbDNBbGxvY2F0aW9uOiAiMHgwMDAzIgogICAgICBDT1MzOgogICAgICAgIGwyQWxsb2NhdGlvbjogIjB4ZmZmZmMiCiAgICAgICAgbDNBbGxvY2F0aW9uOiAiMHgwMDBjIgo=
          mode: 420
          overwrite: true
          path: /etc/rdt-config.yaml
  ```

  the data within the `source` section, starting after `base64,` is the base64 encoded `rdt-config.yaml` file. In this example the file looked as follows:

  ```yaml
  # Common options
  options:
    l2:
      optional: true
    l3:
      optional: true
    mb:
      optional: true
  # Partition definitions
  partitions:
    p1:
      l2Allocation: "0xfffff"
      l3Allocation: "0xfff"
      classes:
        system/default:
          l2Allocation: "0xffffc"
          l3Allocation: "0xff0"
        COS1:
          l2Allocation: "0x00003"
          l3Allocation: "0xff0"
        COS2:
          l2Allocation: "0xffffc"
          l3Allocation: "0x0003"
        COS3:
          l2Allocation: "0xffffc"
          l3Allocation: "0x000c"
  ```

  If another file should be used, one can get the encoded data by means of
  ```bash
  base64 rdt-config.yaml
  ```

- Systemd mount target to mount resctrl sysfs entry

  ```yaml
  # cat << EOF | base64
  # [Unit]
  # Description=Mount resctrl sysfs at /sys/fs/resctrl
  # Before=crio.service
  # 
  # [Mount]
  # What=resctrl
  # Where=/sys/fs/resctrl
  # Type=resctrl
  # Options=noauto,nofail
  # 
  # [Install]
  # WantedBy=multi-user.target crio.service
  # EOF
  apiVersion: machineconfiguration.openshift.io/v1
  kind: MachineConfig
  metadata:
    labels:
      machineconfiguration.openshift.io/role: worker-rt
    name: 90-sys-fs-resctrl-mount
  spec:
    config:
      ignition:
        version: 3.2.0
      storage:
        files:
        - contents:
            source: data:text/plain;charset=utf-8;base64,W1VuaXRdCkRlc2NyaXB0aW9uPU1vdW50ICByZXNjdHJsIHN5c2ZzIGF0IC9zeXMvZnMvcmVzY3RybApCZWZvcmU9Y3Jpby5zZXJ2aWNlCgpbTW91bnRdCldoYXQ9cmVzY3RybApXaGVyZT0vc3lzL2ZzL3Jlc2N0cmwKVHlwZT1yZXNjdHJsCk9wdGlvbnM9bm9hdXRvLG5vZmFpbAoKW0luc3RhbGxdCldhbnRlZEJ5PW11bHRpLXVzZXIudGFyZ2V0IGNyaW8uc2VydmljZQo=
          mode: 420
          overwrite: true
          path: /etc/systemd/system/sys-fs-resctrl.mount
  ```

The MCO will take care of updating and rebooting the nodes.

## SCC configuration

The ExCAT device plugin requires to be run in privileged mode. For this to work one has to configure Openshift's Security Context Constraints (SCCs) before deploying the helm chart based on

```bash
oc adm policy add-scc-to-user privileged -z csl-excat-deviceplugin -n excat

```

## Images
To make the images available within the Openshift cluster, one should use the Openshift registry. For development purposes, the images can also be copied into the local container storage based on `skopeo`. For that copy the images to the nodes using e.g. `scp`. Then on all the worker nodes with enabled ExCAT service (node patched with label `excat=yes`), do

```bash
skopeo copy docker-archive:/tmp/csl-excat-deviceplugin.tar containers-storage:localhost/csl-excat-deviceplugin:v0.1.0
skopeo copy docker-archive:/tmp/csl-excat-deviceplugin.tar containers-storage:localhost/csl-excat-init:v0.1.0
```
and on all control plane nodes
```bash
skopeo copy docker-archive:/tmp/csl-excat-deviceplugin.tar containers-storage:localhost/csl-excat-admission:v0.1.0
```