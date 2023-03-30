# Standing up a Talos Linux Cluster on Azure Using Pulumi

This repository contains a [Pulumi](https://www.pulumi.com) program, written in Golang, to automate the process of standing up a [Talos Linux](https://talos.dev) cluster on Azure.

## Prerequisites

Before using the contents of this repository, you will need to ensure:

* You have the Pulumi CLI installed (see [here](https://www.pulumi.com/docs/get-started/install/) for more information on installing Pulumi).
* You have a working Azure CLI installation.
* You have a working installation of Golang.
* You have manually installed the Pulumi provider for Talos. As of this writing, the Pulumi provider for Talos was still prerelease and needs to be installed manually; see instructions [here](https://blog.scottlowe.org/2023/02/08/installing-prerelease-pulumi-provider-talos/).

## Instructions

1. Clone this repository into a directory on your local computer.

2. Change into the directory where you cloned this repository.

3. Run `pulumi stack init` to create a new Pulumi stack.

4. Use `pulumi config set azure:location <region>` to set the desired Azure region in which to create the cluster (like "WestUS2").

5. Use `pulumi config set` to set the ID for a Talos Linux image that you've created from an uploaded VHD file. Refer to [the instructions for running Talos on Azure](https://www.talos.dev/v1.3/talos-guides/install/cloud-platforms/azure/) for more details on creating your own VM image (pay attention to the "Create the Image" section).

6. Run `pulumi up` to run the Pulumi program.

After the Pulumi program finishes running, you can obtain a configuration file for `talosctl` using this command:

```shell
pulumi stack output talosctlCfg --show-secrets > talosconfig
```

Review the contents of the `talosconfig` file you just created (using `less`, `more`, or `cat`), and make note of one of the IP addresses of the control plane VMs.

You can then run this command to watch the cluster bootstrap:

```shell
talosctl --talosconfig talosconfig --nodes <cp-vm-ip-address> health
```

Once the cluster has finished boostrapping, you can retrieve the Kubeconfig necessary to access the cluster with this command:

```shell
talosctl --talosconfig talosconfig --nodes <cp-vm-ip-address> kubeconfig
```

You can then use `kubectl` to access the cluster as normal, referencing the recently-retrieved Kubeconfig as necessary.
