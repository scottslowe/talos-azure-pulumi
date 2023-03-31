package main

import (
	"fmt"
	"log"

	"github.com/pulumi/pulumi-azure/sdk/v5/go/azure/compute"
	"github.com/pulumi/pulumi-azure/sdk/v5/go/azure/core"
	"github.com/pulumi/pulumi-azure/sdk/v5/go/azure/lb"
	"github.com/pulumi/pulumi-azure/sdk/v5/go/azure/network"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
	"github.com/siderolabs/pulumi-provider-talos/sdk/go/talos"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Get some configuration values
		cfg := config.New(ctx, "")
		talosImageId := cfg.Require("imageId")

		// Create a resource group for the Talos Linux resources
		// Details: https://www.pulumi.com/registry/packages/azure/api-docs/core/resourcegroup/
		talosRg, err := core.NewResourceGroup(ctx, "talos-rg", &core.ResourceGroupArgs{
			Name: pulumi.String("talos-rg"),
		})
		if err != nil {
			log.Printf("error creating resource group: %s", err.Error())
		}

		// Create a virtual network
		// Details: https://www.pulumi.com/registry/packages/azure/api-docs/network/virtualnetwork/
		talosVnet, err := network.NewVirtualNetwork(ctx, "talos-vnet", &network.VirtualNetworkArgs{
			AddressSpaces: pulumi.StringArray{
				pulumi.String("10.0.0.0/16"),
			},
			Name:              pulumi.String("talos-vnet"),
			ResourceGroupName: talosRg.Name,
		})
		if err != nil {
			log.Printf("error creating virtual network: %s", err.Error())
		}

		// Create three subnets
		// Details: https://www.pulumi.com/registry/packages/azure/api-docs/network/subnet/
		talosSubnetIds := make([]pulumi.StringInput, 3)
		for i := 0; i < 3; i++ {
			subnet, err := network.NewSubnet(ctx, fmt.Sprintf("subnet-0%d", i+1), &network.SubnetArgs{
				AddressPrefixes: pulumi.StringArray{
					pulumi.Sprintf("10.0.%d.0/24", i+1),
				},
				ResourceGroupName:  talosRg.Name,
				VirtualNetworkName: talosVnet.Name,
			})
			if err != nil {
				log.Printf("error creating subnet: %s", err.Error())
			}
			talosSubnetIds[i] = subnet.ID()
		}

		// Create a network security group and associated rules
		// Details: https://www.pulumi.com/registry/packages/azure/api-docs/network/networksecuritygroup/
		talosNsg, err := network.NewNetworkSecurityGroup(ctx, "talos-nsg", &network.NetworkSecurityGroupArgs{
			Name:              pulumi.String("talos-nsg"),
			ResourceGroupName: talosRg.Name,
			SecurityRules: &network.NetworkSecurityGroupSecurityRuleArray{
				&network.NetworkSecurityGroupSecurityRuleArgs{
					Access:                   pulumi.String("Allow"),
					DestinationAddressPrefix: pulumi.String("*"),
					DestinationPortRange:     pulumi.String("50000"),
					Direction:                pulumi.String("Inbound"),
					Name:                     pulumi.String("apid"),
					Priority:                 pulumi.Int(1001),
					Protocol:                 pulumi.String("Tcp"),
					SourceAddressPrefix:      pulumi.String("*"),
					SourcePortRange:          pulumi.String("*"),
				},
				&network.NetworkSecurityGroupSecurityRuleArgs{
					Access:                   pulumi.String("Allow"),
					DestinationAddressPrefix: pulumi.String("*"),
					DestinationPortRange:     pulumi.String("50001"),
					Direction:                pulumi.String("Inbound"),
					Name:                     pulumi.String("trustd"),
					Priority:                 pulumi.Int(1002),
					Protocol:                 pulumi.String("Tcp"),
					SourceAddressPrefix:      pulumi.String("*"),
					SourcePortRange:          pulumi.String("*"),
				},
				&network.NetworkSecurityGroupSecurityRuleArgs{
					Access:                   pulumi.String("Allow"),
					DestinationAddressPrefix: pulumi.String("*"),
					DestinationPortRange:     pulumi.String("2379-2380"),
					Direction:                pulumi.String("Inbound"),
					Name:                     pulumi.String("etcd"),
					Priority:                 pulumi.Int(1003),
					Protocol:                 pulumi.String("Tcp"),
					SourceAddressPrefix:      pulumi.String("*"),
					SourcePortRange:          pulumi.String("*"),
				},
				&network.NetworkSecurityGroupSecurityRuleArgs{
					Access:                   pulumi.String("Allow"),
					DestinationAddressPrefix: pulumi.String("*"),
					DestinationPortRange:     pulumi.String("6443"),
					Direction:                pulumi.String("Inbound"),
					Name:                     pulumi.String("k8s"),
					Priority:                 pulumi.Int(1004),
					Protocol:                 pulumi.String("Tcp"),
					SourceAddressPrefix:      pulumi.String("*"),
					SourcePortRange:          pulumi.String("*"),
				},
			},
		})
		if err != nil {
			log.Printf("error creating network security group: %s", err.Error())
		}

		// Create a public IP address for the load balancer
		// Details: https://www.pulumi.com/registry/packages/azure/api-docs/network/publicip/
		talosLbPubIp, err := network.NewPublicIp(ctx, "talos-lb-pub-ip", &network.PublicIpArgs{
			AllocationMethod:  pulumi.String("Static"),
			Name:              pulumi.String("talos-lb-pub-ip"),
			ResourceGroupName: talosRg.Name,
			Sku:               pulumi.String("Standard"),
		})
		if err != nil {
			log.Printf("error allocating public IP: %s", err.Error())
		}

		// Create a load balancer (only used for K8s API traffic)
		// Details: https://www.pulumi.com/registry/packages/azure/api-docs/lb/loadbalancer/
		talosLb, err := lb.NewLoadBalancer(ctx, "talos-lb", &lb.LoadBalancerArgs{
			FrontendIpConfigurations: &lb.LoadBalancerFrontendIpConfigurationArray{
				&lb.LoadBalancerFrontendIpConfigurationArgs{
					Name:              pulumi.String("talos-fe-ip"),
					PublicIpAddressId: talosLbPubIp.ID(),
				},
			},
			Name:              pulumi.String("talos-lb"),
			ResourceGroupName: talosRg.Name,
			Sku:               pulumi.String("Standard"),
		})
		if err != nil {
			log.Printf("error creating load balancer: %s", err.Error())
		}

		// Create a backend address pool
		// Details: https://www.pulumi.com/registry/packages/azure/api-docs/lb/backendaddresspool/
		talosBePool, err := lb.NewBackendAddressPool(ctx, "talos-be-pool", &lb.BackendAddressPoolArgs{
			LoadbalancerId: talosLb.ID(),
			Name:           pulumi.String("talos-be-pool"),
		})
		if err != nil {
			log.Printf("error creating backend address pool: %s", err.Error())
		}

		// Create a health probe for the load balancer
		// Details: https://www.pulumi.com/registry/packages/azure/api-docs/lb/probe/
		_, err = lb.NewProbe(ctx, "talos-lb-probe", &lb.ProbeArgs{
			LoadbalancerId: talosLb.ID(),
			Port:           pulumi.Int(6443),
			Name:           pulumi.String("talos-lb-probe"),
			Protocol:       pulumi.String("Tcp"),
		})
		if err != nil {
			log.Printf("error creating load balancer probe: %s", err.Error())
		}

		// Create a load balancing rule for K8s API traffic
		// Details: https://www.pulumi.com/registry/packages/azure/api-docs/lb/rule/
		_, err = lb.NewRule(ctx, "talos-lb-rule", &lb.RuleArgs{
			DisableOutboundSnat:         pulumi.Bool(true),
			EnableTcpReset:              pulumi.Bool(true),
			LoadbalancerId:              talosLb.ID(),
			Protocol:                    pulumi.String("Tcp"),
			FrontendPort:                pulumi.Int(6443),
			BackendPort:                 pulumi.Int(6443),
			FrontendIpConfigurationName: pulumi.String("talos-fe-ip"),
			BackendAddressPoolIds:       pulumi.StringArray{talosBePool.ID()},
		})
		if err != nil {
			log.Printf("error creating load balancer rule: %s", err.Error())
		}

		// Create public IPs and network interfaces for control plane machines
		cpPublicIps := make([]pulumi.StringInput, 3)
		cpPublicIpIds := make([]pulumi.StringInput, 3)
		cpIntfIds := make([]pulumi.StringInput, 3)
		for i := 0; i < 3; i++ {
			// Create the public IP address
			publicIp, err := network.NewPublicIp(ctx, fmt.Sprintf("cp-pub-ip-0%d", i+1), &network.PublicIpArgs{
				AllocationMethod:  pulumi.String("Static"),
				Name:              pulumi.Sprintf("cp-pub-ip-0%d", i+1),
				ResourceGroupName: talosRg.Name,
				Sku:               pulumi.String("Standard"),
			})
			if err != nil {
				log.Printf("error allocating public IP: %s", err.Error())
			}
			cpPublicIps[i] = publicIp.IpAddress
			cpPublicIpIds[i] = publicIp.ID()

			// Create the network interface
			// Details: https://www.pulumi.com/registry/packages/azure/api-docs/network/networkinterface/
			ni, err := network.NewNetworkInterface(ctx, fmt.Sprintf("cp-ni-0%d", i+1), &network.NetworkInterfaceArgs{
				IpConfigurations: &network.NetworkInterfaceIpConfigurationArray{
					&network.NetworkInterfaceIpConfigurationArgs{
						Name:                       pulumi.Sprintf("cp-ni-0%d", i+1),
						SubnetId:                   talosSubnetIds[i],
						PrivateIpAddressAllocation: pulumi.String("Dynamic"),
						PublicIpAddressId:          cpPublicIpIds[i],
					},
				},
				ResourceGroupName: talosRg.Name,
			})
			if err != nil {
				log.Printf("error creating network interface: %s", err.Error())
			}
			cpIntfIds[i] = ni.ID()

			// Associate the NI to the NSG
			// Details: https://www.pulumi.com/registry/packages/azure/api-docs/network/networkinterfacesecuritygroupassociation/
			_, err = network.NewNetworkInterfaceSecurityGroupAssociation(ctx, fmt.Sprintf("ni-nsg-0%d", i+1), &network.NetworkInterfaceSecurityGroupAssociationArgs{
				NetworkInterfaceId:     ni.ID(),
				NetworkSecurityGroupId: talosNsg.ID(),
			})
			if err != nil {
				log.Printf("error associating interface with security group: %s", err.Error())
			}

			// Add the NI to the backend address pool
			// Details: https://www.pulumi.com/registry/packages/azure/api-docs/network/networkinterfacebackendaddresspoolassociation/
			_, err = network.NewNetworkInterfaceBackendAddressPoolAssociation(ctx, fmt.Sprintf("ni-bepool-0%d", i+1), &network.NetworkInterfaceBackendAddressPoolAssociationArgs{
				BackendAddressPoolId: talosBePool.ID(),
				IpConfigurationName:  pulumi.Sprintf("cp-ni-0%d", i+1),
				NetworkInterfaceId:   ni.ID(),
			})
			if err != nil {
				log.Printf("error associating interface with backend address pool: %s", err.Error())
			}
		}

		// Start building the Talos configuration by generating new machine secrets
		// Details: https://github.com/siderolabs/pulumi-provider-talos/blob/main/sdk/go/talos/talosMachineSecrets.go
		talosMs, err := talos.NewTalosMachineSecrets(ctx, "talos-ms", nil)
		if err != nil {
			log.Printf("error creating machine secrets: %s", err.Error())
		}

		// Create the talosctl configuration file
		// Details: https://github.com/siderolabs/pulumi-provider-talos/blob/main/sdk/go/talos/talosClientConfiguration.go
		talosClientCfg, err := talos.NewTalosClientConfiguration(ctx, "talos-client-cfg", &talos.TalosClientConfigurationArgs{
			ClusterName:    pulumi.String("talos-cluster"),
			MachineSecrets: talosMs.MachineSecrets,
			Endpoints:      pulumi.StringArray(cpPublicIps),
			Nodes:          pulumi.StringArray(cpPublicIps),
		})
		if err != nil {
			log.Printf("error creating client configuration: %s", err.Error())
		}

		// Create the machine configuration for the control plane VMs
		// Details: https://github.com/siderolabs/pulumi-provider-talos/blob/main/sdk/go/talos/talosMachineConfigurationControlplane.go
		talosCpMachineCfg, err := talos.NewTalosMachineConfigurationControlplane(ctx, "talos-cp-machine-cfg", &talos.TalosMachineConfigurationControlplaneArgs{
			ClusterName:     talosClientCfg.ClusterName,
			ClusterEndpoint: pulumi.Sprintf("https://%v:6443", talosLbPubIp.IpAddress),
			MachineSecrets:  talosMs.MachineSecrets,
			DocsEnabled:     pulumi.Bool(false),
			ExamplesEnabled: pulumi.Bool(false),
		})
		if err != nil {
			log.Printf("error creating machine configuration: %s", err.Error())
		}

		// Create the machine configuration for the worker VMs
		// Details: https://github.com/siderolabs/pulumi-provider-talos/blob/main/sdk/go/talos/talosMachineConfigurationWorker.go
		talosWkrMachineCfg, err := talos.NewTalosMachineConfigurationWorker(ctx, "talos-wkr-machine-cfg", &talos.TalosMachineConfigurationWorkerArgs{
			ClusterName:     talosClientCfg.ClusterName,
			ClusterEndpoint: pulumi.Sprintf("https://%v:6443", talosLbPubIp.IpAddress),
			MachineSecrets:  talosMs.MachineSecrets,
			DocsEnabled:     pulumi.Bool(false),
			ExamplesEnabled: pulumi.Bool(false),
		})
		if err != nil {
			log.Printf("error creating machine configuration: %s", err.Error())
		}

		// Create an availability set for the control plane
		// Details: https://www.pulumi.com/registry/packages/azure/api-docs/compute/availabilityset/
		talosCpAvailSet, err := compute.NewAvailabilitySet(ctx, "talos-cp-avail-set", &compute.AvailabilitySetArgs{
			ResourceGroupName: talosRg.Name,
		})
		if err != nil {
			log.Printf("error creating availability set: %s", err.Error())
		}

		// Launch the VMs for the control plane
		// Details: https://www.pulumi.com/registry/packages/azure/api-docs/compute/virtualmachine/
		cpVmIds := make([]pulumi.StringInput, 3)
		for i := 0; i < 3; i++ {
			vm, err := compute.NewVirtualMachine(ctx, fmt.Sprintf("talos-cp-0%d", i+1), &compute.VirtualMachineArgs{
				AvailabilitySetId:         talosCpAvailSet.ID(),
				DeleteOsDiskOnTermination: pulumi.Bool(true),
				NetworkInterfaceIds: pulumi.StringArray{
					cpIntfIds[i],
				},
				OsProfile: &compute.VirtualMachineOsProfileArgs{
					ComputerName:  pulumi.Sprintf("talos-cp-0%d", i+1),
					AdminUsername: pulumi.String("talosadmin"),    // ignored by Talos
					AdminPassword: pulumi.String("Password1234!"), // ignored by Talos
					CustomData:    talosCpMachineCfg.MachineConfig,
				},
				OsProfileLinuxConfig: &compute.VirtualMachineOsProfileLinuxConfigArgs{
					DisablePasswordAuthentication: pulumi.Bool(false),
				},
				ResourceGroupName: talosRg.Name,
				StorageImageReference: &compute.VirtualMachineStorageImageReferenceArgs{
					Id: pulumi.String(talosImageId),
				},
				StorageOsDisk: &compute.VirtualMachineStorageOsDiskArgs{
					Name:            pulumi.Sprintf("talos-cp-0%d-disk", i+1),
					Caching:         pulumi.String("ReadWrite"),
					CreateOption:    pulumi.String("FromImage"),
					ManagedDiskType: pulumi.String("Standard_LRS"),
					DiskSizeGb:      pulumi.Int(30),
				},
				VmSize: pulumi.String("Standard_DS1_v2"),
			})
			if err != nil {
				log.Printf("error creating virtual machine: %s", err.Error())
			}
			cpVmIds[i] = vm.ID()
		}

		// Create network interfaces for the worker node VMs
		wkrIntfIds := make([]pulumi.StringInput, 3)
		for i := 0; i < 3; i++ {
			// Create the network interface
			ni, err := network.NewNetworkInterface(ctx, fmt.Sprintf("wkr-ni-0%d", i+1), &network.NetworkInterfaceArgs{
				IpConfigurations: &network.NetworkInterfaceIpConfigurationArray{
					&network.NetworkInterfaceIpConfigurationArgs{
						Name:                       pulumi.Sprintf("wkr-ni-0%d", i+1),
						SubnetId:                   talosSubnetIds[i],
						PrivateIpAddressAllocation: pulumi.String("Dynamic"),
					},
				},
				ResourceGroupName: talosRg.Name,
			})
			if err != nil {
				log.Printf("error creating network interface: %s", err.Error())
			}
			wkrIntfIds[i] = ni.ID()
		}

		// Launch the VMs for the worker nodes
		wkrVmIds := make([]pulumi.StringInput, 3)
		for i := 0; i < 3; i++ {
			vm, err := compute.NewVirtualMachine(ctx, fmt.Sprintf("talos-wkr-0%d", i+1), &compute.VirtualMachineArgs{
				DeleteOsDiskOnTermination: pulumi.Bool(true),
				NetworkInterfaceIds: pulumi.StringArray{
					wkrIntfIds[i],
				},
				OsProfile: &compute.VirtualMachineOsProfileArgs{
					ComputerName:  pulumi.Sprintf("talos-wkr-0%d", i+1),
					AdminUsername: pulumi.String("talosadmin"),    // ignored by Talos
					AdminPassword: pulumi.String("Password1234!"), // ignored by Talos
					CustomData:    talosWkrMachineCfg.MachineConfig,
				},
				OsProfileLinuxConfig: &compute.VirtualMachineOsProfileLinuxConfigArgs{
					DisablePasswordAuthentication: pulumi.Bool(false),
				},
				ResourceGroupName: talosRg.Name,
				StorageImageReference: &compute.VirtualMachineStorageImageReferenceArgs{
					Id: pulumi.String(talosImageId),
				},
				StorageOsDisk: &compute.VirtualMachineStorageOsDiskArgs{
					Name:            pulumi.Sprintf("talos-wkr-0%d-disk", i+1),
					Caching:         pulumi.String("ReadWrite"),
					CreateOption:    pulumi.String("FromImage"),
					ManagedDiskType: pulumi.String("Standard_LRS"),
					DiskSizeGb:      pulumi.Int(30),
				},
				VmSize: pulumi.String("Standard_DS1_v2"),
			})
			if err != nil {
				log.Printf("error creating virtual machine: %s", err.Error())
			}
			wkrVmIds[i] = vm.ID()
		}

		// Bootstrap the first control plane node
		// Details: https://github.com/siderolabs/pulumi-provider-talos/blob/main/sdk/go/talos/talosMachineBootstrap.go
		_, err = talos.NewTalosMachineBootstrap(ctx, "bootstrap", &talos.TalosMachineBootstrapArgs{
			TalosConfig: talosClientCfg.TalosConfig,
			Endpoint:    cpPublicIps[0],
			Node:        cpPublicIps[0],
		})
		if err != nil {
			log.Printf("error encountered during bootstrap: %s", err.Error())
		}

		// Export the Talos client configuration
		ctx.Export("talosctlCfg", talosClientCfg.TalosConfig)

		// Uncomment the following lines for additional outputs that may be useful for troubleshooting/diagnostics
		// ctx.Export("talosKubeLbIp", talosLbPubIp.IpAddress)
		// ctx.Export("talosCpMachineConfig", talosCpMachineCfg.MachineConfig)
		// ctx.Export("talosWkrMachineConfig", talosWkrMachineCfg.MachineConfig)
		// ctx.Export("talosSecrets", talosMs.MachineSecrets)

		return nil
	})
}
