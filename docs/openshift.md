# Introduction:
For the moment Excat service does not provide an operator that automates the installation on an Openshift cluster. The following instructions need to be followed.

# Limitation: 
With the latest Openshift 4.10 there is a bug in runc causing failure when allocating from L2 cache.
runc is still at version: runc-1.1.0-2.rhaos4.10.el8.x86_64
https://github.com/opencontainers/runc/pull/3382
This has been fixed in runc master targeting 1.2 so it's not yet available with the lastest Openshift update.

# Node configuration
Required kernel configs for excat are only supported on openshift realtime kernel config.
Performance Addon Operator need to be used to enable that.
https://docs.openshift.com/container-platform/4.10/scalability_and_performance/cnf-performance-addon-operator-for-low-latency-nodes.html

In addition restrl sysfs need to be mounted, crio and rdt need to be configured. This can be done manually (not recommended) or using MCO (Machine Config Operator) for
more details see:
https://docs.openshift.com/container-platform/4.10/post_installation_configuration/machine-configuration-tasks.html#understanding-the-machine-config-operator

Create the following machine configs for resctrl mount, CRIO and rdt config:

- CRIO-O configuration to enable RDT and allow the required annotations:
 
 ```
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
  generation: 1
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

 ```
#cat << EOF | base64
#[Unit]
#Requires=sys-fs-resctrl.mount
#
#EOF


apiVersion: machineconfiguration.openshift.io/v1
kind: MachineConfig
metadata:
  generation: 1
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
 
 ```
apiVersion: machineconfiguration.openshift.io/v1
kind: MachineConfig
metadata:
  creationTimestamp: "2022-08-03T12:56:24Z"
  generation: 1
  name: rendered-worker-rt-6202983988b1f8480db45bd6f7780bbd
  resourceVersion: "15037979"
  uid: 2179d403-6380-4675-b21b-dfda6fd98564
spec: {}
apiVersion: machineconfiguration.openshift.io/v1
kind: MachineConfig
metadata:
  generation: 1
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
          source: data:text/plain;charset=utf-8;base64,IyBDb21tb24gb3B0aW9ucwpvcHRpb25zOgogIGwyOgogICAgIyBTZXQgdG8gZmFsc2UgaWYgTDIgQ0FUIG11c3QgYmUgYXZhaWxhYmxlIChEZWZhdWx0IGlzIHRydWUpLgogICAgb3B0aW9uYWw6IHRydWUKICBsMzoKICAgICMgU2V0IHRvIGZhbHNlIGlmIEwzIENBVCBtdXN0IGJlIGF2YWlsYWJsZSAoRGVmYXVsdCBpcyB0cnVlKS4KICAgIG9wdGlvbmFsOiB0cnVlCiAgbWI6CiAgICAjIFNldCB0byBmYWxzZSBpZiBNQkEgbXVzdCBiZSBhdmFpbGFibGUgKERlZmF1bHQgaXMgdHJ1ZSkuCiAgICBvcHRpb25hbDogdHJ1ZQojIFBhcnRpdGlvbiBkZWZpbml0aW9ucwpwYXJ0aXRpb25zOgogIHBkZWY6CiAgICBsMkFsbG9jYXRpb246ICI1MCUiCiAgICBsM0FsbG9jYXRpb246ICI1MCUiCiAgICBjbGFzc2VzOgogICAgICBzeXN0ZW0vZGVmYXVsdDoKICAgICAgICBsMnNjaGVtYTogIjEwMCUiCiAgICAgICAgbDNzY2hlbWE6ICIxMDAlIgogIHAwOgogICAgbDJBbGxvY2F0aW9uOiAiMjUlIgogICAgbDNBbGxvY2F0aW9uOiAiMjUlIgogICAgY2xhc3NlczoKICAgICAgYzA6CiAgICAgICAgbDJzY2hlbWE6ICIxMDAlIgogICAgICAgIGwzc2NoZW1hOiAiMTAwJSIKICBwMToKICAgIGwyQWxsb2NhdGlvbjogIjI1JSIKICAgIGwzQWxsb2NhdGlvbjogIjI1JSIKICAgIGNsYXNzZXM6CiAgICAgIGMxOgogICAgICAgIGwyc2NoZW1hOiAiMTAwJSIKICAgICAgICBsM3NjaGVtYTogIjEwMCUiCg==
        mode: 420
        overwrite: true
        path: /etc/rdt-config.yaml
 ```


 - Systemd mount target to mount resctrl sysfs entry

 ```
 # cat << EOF | base64
 # [Unit]
 # Description=Mount  resctrl sysfs at /sys/fs/resctrl
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
  generation: 1
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
MCO will take care of updating and rebooting the nodes.

# SCC configuration
Excat device plugin is privileged! so before applying the helm chart make sure to configure SCC. 

```
oc adm policy add-scc-to-user privileged -z csl-excat-deviceplugin

```
