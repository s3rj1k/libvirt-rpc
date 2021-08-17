package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/libvirt/libvirt-go"
	"github.com/libvirt/libvirt-go-xml"
)

func findNodePFName(ctx context.Context, c *libvirt.Connect, pfPciName string) (string, error) {

	id := getReqIDFromContext(ctx)

	var err error
	var pciName, pciParentName, pfName string

	devs, err := listNodeNetworkDevices(ctx, c)
	defer freeNodeDevices(ctx, devs)
	if err != nil {
		return "", err
	}

	for _, dev := range devs {

		xml, err := dev.GetXMLDesc(0)
		if err != nil {
			fail.Printf("%sfailed to get device XML: %s\n", id, err.Error())
			continue
		}

		nodeDev := &libvirtxml.NodeDevice{}
		err = nodeDev.Unmarshal(xml)
		if err != nil {
			fail.Printf("%sfailed to unmarshal XML: %s\n", id, err.Error())
			continue
		}

		if nodeDev != nil {
			pciName = nodeDev.Name
			pciParentName = nodeDev.Parent
			if nodeDev.Capability.Net != nil {
				if nodeDev.Capability.Net.Interface != "" {
					pfName = nodeDev.Capability.Net.Interface
					info.Printf("%sfound node network PCI device: %s; PCI device parent: %s; network device name: %s\n", id, pciName, pciParentName, pciName)
				}
			}
		}

		if pciParentName == pfPciName {
			break
		}

	}

	if len(pfName) == 0 {
		fail.Printf("%sfailed to get node network device name: %s\n", id, errors.New("device not found"))
		return "", errors.New("device not found")
	}

	return pfName, nil
}

func getNodePFInfo(ctx context.Context, c *libvirt.Connect, vfPciName string) (string, string, string) {

	id := getReqIDFromContext(ctx)

	var pfPciName, pfPci, desc string

	dev, err := lookupNodeDeviceByName(ctx, c, vfPciName)
	defer freeNodeDevice(ctx, dev)
	if err != nil || dev == nil {
		return "", "", ""
	}

	xml, err := dev.GetXMLDesc(0)
	if err != nil {
		fail.Printf("%sfailed to get PCI device %s XML: %s\n", id, vfPciName, err.Error())
		return "", "", ""
	}
	info.Printf("%sacquired PCI device %s XML\n", id, vfPciName)

	devCfg := &libvirtxml.NodeDevice{}
	err = devCfg.Unmarshal(xml)
	if err != nil {
		fail.Printf("%sfailed to parse PCI device %s XML: %s\n", id, vfPciName, err.Error())
		return "", "", ""
	}
	info.Printf("%sparsed PCI device %s XML\n", id, vfPciName)

	if devCfg.Capability.PCI == nil {
		fail.Printf("%sfailed to parse PCI device %s XML: %s\n", id, vfPciName, errors.New("device capability not found"))
		return "", "", ""
	}

	desc = fmt.Sprintf("%s %s", devCfg.Capability.PCI.Vendor.Name, devCfg.Capability.PCI.Product.Name)

	for _, cap := range devCfg.Capability.PCI.Capabilities {
		if cap.PhysFunction != nil {
			pfPci = fmt.Sprintf("%04x:%02x:%02x.%x",
				*cap.PhysFunction.Address.Domain,
				*cap.PhysFunction.Address.Bus,
				*cap.PhysFunction.Address.Slot,
				*cap.PhysFunction.Address.Function)
			pfPciName = fmt.Sprintf("pci_%04x_%02x_%02x_%x",
				*cap.PhysFunction.Address.Domain,
				*cap.PhysFunction.Address.Bus,
				*cap.PhysFunction.Address.Slot,
				*cap.PhysFunction.Address.Function)
			info.Printf("%sPCI device %s has phys_function\n", id, vfPciName)
			break
		}
	}

	if len(pfPci) == 0 || len(pfPciName) == 0 {
		fail.Printf("%sphys_function section not found in PCI device %s XML.", id, vfPciName)
		return "", "", ""
	}

	return pfPciName, pfPci, desc
}

func getNetworkVFCount(ctx context.Context, c *libvirt.Connect, n *libvirt.Network) (int, int, error) {

	netHostDevPciDevices, err := getInterfacesFromNetwork(ctx, n)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get list of interfaces: %s", err.Error())
	}

	usedVFs := 0
	totalVFs := len(netHostDevPciDevices) - 1

	netDomainPciDevices, err := getDomainsNetworkDevices(ctx, c)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get list of pci devices: %s", err.Error())
	}

	for _, hostDevAddr := range netHostDevPciDevices {
		for _, domainAddr := range netDomainPciDevices {
			if hostDevAddr == domainAddr {
				usedVFs = usedVFs + 1
			}
		}
	}

	return usedVFs, totalVFs, nil
}

func getNetworkNameForVFAddress(ctx context.Context, c *libvirt.Connect, vfAddr string) (string, error) {

	id := getReqIDFromContext(ctx)

	var name string

	networks, err := listNetworks(ctx, c)
	defer freeNetworks(ctx, networks)
	if err != nil {
		return "", err
	}

NetworksLoop:
	for _, net := range networks {

		var netHostDevPciDevices []string
		var err error

		name, err = getNetworkName(ctx, &net)
		if err != nil {
			continue
		}

		netHostDevPciDevices, err = getInterfacesFromNetwork(ctx, &net)
		if err != nil {
			continue
		}

		for _, hostDevAddr := range netHostDevPciDevices {
			if hostDevAddr == vfAddr {
				info.Printf("%sacquired defined network name %s for %s\n", id, name, vfAddr)
				break NetworksLoop
			}
		}

	}

	if len(name) == 0 {
		fail.Printf("%sfailed to find defined network for %s: %s\n", id, vfAddr, errors.New("no defined network matches search criteria"))
		return "", errors.New("no defined network matches search criteria")
	}

	return name, nil
}

func getInterfacesFromNetwork(ctx context.Context, net *libvirt.Network) ([]string, error) {

	id := getReqIDFromContext(ctx)

	netHostDevPciDevices := make([]string, 0)

	xml, err := net.GetXMLDesc(libvirt.NetworkXMLFlags(0))
	if err != nil {
		fail.Printf("%sfailed to get XML description of defined network: %s\n", id, err.Error())
		return []string{}, err
	}
	info.Printf("%sacquired XML description of defined network\n", id)

	netCfg := &libvirtxml.Network{}
	err = netCfg.Unmarshal(xml)
	if err != nil {
		fail.Printf("%sfailed to parse Network XML: %s\n", id, err.Error())
		return []string{}, err
	}
	info.Printf("%sparsed Network XML\n", id)

	if netCfg != nil {
		if netCfg.Forward != nil {
			if strings.ToLower(netCfg.Forward.Mode) == "hostdev" && strings.ToLower(netCfg.Forward.Managed) == yes {
				if netCfg.Forward.Driver != nil {
					if strings.ToLower(netCfg.Forward.Driver.Name) == "vfio" || strings.ToLower(netCfg.Forward.Driver.Name) == "kvm" {
						for _, dev := range netCfg.Forward.Addresses {
							vfPciAddr := fmt.Sprintf("%04x:%02x:%02x.%x",
								*dev.PCI.Domain,
								*dev.PCI.Bus,
								*dev.PCI.Slot,
								*dev.PCI.Function)
							info.Printf("%sfound defined host network device %s inside defined network group\n", id, vfPciAddr)
							netHostDevPciDevices = append(netHostDevPciDevices, vfPciAddr)
						}
					}
				}
			}
		}
	}

	info.Printf("%sacquired all host network devices inside defined network group\n", id)
	return netHostDevPciDevices, nil
}

func isNetworkVFAvailable(ctx context.Context, c *libvirt.Connect, network string) (bool, error) {

	id := getReqIDFromContext(ctx)

	if len(network) == 0 {
		fail.Printf("%snetwork name can not be empty\n", id)
		return false, fmt.Errorf("network name can not be empty")
	}

	// ToDo: change naming convention for network -> pf-port105 {SWITCHNUM/PORTNUM}
	net, err := lookupNetworkByName(ctx, c, network)
	if err != nil {
		return false, fmt.Errorf("network %s does not exist: %s", network, err.Error())
	}

	usedVFs, totalVFs, err := getNetworkVFCount(ctx, c, net)
	if err != nil {
		return false, fmt.Errorf("failed to get list of interfaces inside network %s: %s", network, err.Error())
	}

	if usedVFs >= totalVFs {
		fail.Printf("%sno empty network VF available\n", id)
		return false, fmt.Errorf("no empty network VF available")
	}

	info.Printf("%sVF(s): %d available\n", id, totalVFs-usedVFs)
	return true, nil
}
