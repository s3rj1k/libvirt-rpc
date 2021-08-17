package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/libvirt/libvirt-go"
	"github.com/libvirt/libvirt-go-xml"
)

func isInterfacePassthroughPCIDevice(ctx context.Context, dev libvirtxml.DomainInterface) bool {

	id := getReqIDFromContext(ctx)

	if dev.Source != nil && dev.MAC != nil && strings.ToLower(dev.Managed) == yes {
		if dev.Source.Hostdev != nil {
			if dev.Source.Hostdev.PCI != nil {
				if dev.Source.Hostdev.PCI.Address != nil {
					info.Printf("%snetwork device is host passthrough PCI device\n", id)
					return true
				}
			}
		}
	}

	fail.Printf("%sdevice is not network host passthrough PCI device\n", id)
	return false
}

func getDomainInterfaces(ctx context.Context, d *libvirt.Domain) ([]libvirtxml.DomainInterface, error) {

	id := getReqIDFromContext(ctx)

	xml, err := d.GetXMLDesc(0)
	if err != nil {
		fail.Printf("%sfailed to get Domain XML: %s\n", id, err.Error())
		return []libvirtxml.DomainInterface{}, err
	}
	info.Printf("%sacquired Domain XML\n", id)

	domCfg := &libvirtxml.Domain{}
	err = domCfg.Unmarshal(xml)
	if err != nil {
		fail.Printf("%sfailed to parse Domain XML: %s\n", id, err.Error())
		return []libvirtxml.DomainInterface{}, err
	}
	info.Printf("%sparsed Domain XML\n", id)

	if domCfg.Devices == nil {
		fail.Printf("%sfailed to parse Domain XML: %s\n", id, errors.New("devices XML section is empty"))
		return []libvirtxml.DomainInterface{}, errors.New("devices XML section is empty")
	}

	if domCfg.Devices.Interfaces == nil {
		fail.Printf("%sfailed to parse Domain XML: %s\n", id, errors.New("no interfaces in device XML section"))
		return []libvirtxml.DomainInterface{}, errors.New("no interfaces in device XML section")
	}

	info.Printf("%sacquired network devices structure\n", id)
	return domCfg.Devices.Interfaces, nil
}

func getDomainsNetworkDevices(ctx context.Context, c *libvirt.Connect) ([]string, error) {

	id := getReqIDFromContext(ctx)

	netDomainPciDevices := make([]string, 0)

	domains, err := listAllDomainsWithFlags(ctx, c, libvirt.ConnectListAllDomainsFlags(0))
	defer freeDomains(ctx, domains)
	if err != nil {
		return []string{}, err
	}

	for _, d := range domains {

		interfaces, err := getDomainInterfaces(ctx, &d)
		if err != nil {
			continue
		}

		for _, dev := range interfaces {
			ok := isInterfacePassthroughPCIDevice(ctx, dev)
			if !ok {
				continue
			}
			vfPci := fmt.Sprintf("%04x:%02x:%02x.%x",
				*dev.Source.Hostdev.PCI.Address.Domain,
				*dev.Source.Hostdev.PCI.Address.Bus,
				*dev.Source.Hostdev.PCI.Address.Slot,
				*dev.Source.Hostdev.PCI.Address.Function)
			netDomainPciDevices = append(netDomainPciDevices, vfPci)
		}
	}

	info.Printf("%sacquired all used hostdev network devices\n", id)
	return netDomainPciDevices, nil
}

func setPVIDForDomainNetworkDevice(ctx context.Context, d *libvirt.Domain, mac string, pvid uint) (bool, error) {

	// Achtung: only for domain in shutdown state!

	id := getReqIDFromContext(ctx)

	var nicOld, nicNew libvirtxml.DomainInterface
	var vlan libvirtxml.DomainInterfaceVLan
	var xmlOld, xmlNew string
	var err error

	ok, err := isMACvalid(ctx, mac)
	if err != nil || !ok {
		return false, err
	}

	isActive := isDomainActive(ctx, d)
	if isActive {
		return false, errors.New("domain must not be active while setting PVID for network device")
	}

	c, err := getConnectFromDomain(ctx, d)
	defer closeConnection(ctx, c)
	if err != nil {
		return false, err
	}

	flags := libvirt.DOMAIN_DEVICE_MODIFY_CURRENT

	nics, err := getDomainInterfaces(ctx, d)
	if err != nil {
		return false, err
	}

	vlan.Trunk = ""
	vlan.Tags = make([]libvirtxml.DomainInterfaceVLanTag, 1)
	vlan.Tags[0].ID = pvid
	vlan.Tags[0].NativeMode = ""
	info.Printf("%sprepared XML with VLAN PVID=%d definition\n", id, pvid)

	for _, dev := range nics {
		if dev.MAC != nil {
			if dev.MAC.Address == mac {
				nicOld = dev
				break
			}
		}
	}

	if nicOld.Source == nil {
		return false, errors.New("malformed XML description for interface")
	}

	if nicOld.Source.Network == nil {
		return false, errors.New("malformed XML description for interface")
	}

	if !strings.HasPrefix(nicOld.Source.Network.Network, "pf-") {
		return false, errors.New("malformed XML description for interface")
	}

	if nicOld.MAC == nil {
		fail.Printf("%sfailed to find network device(%s) in domain config: %s\n", id, mac, errors.New("no interfaces match specified MAC"))
		return false, errors.New("no interfaces match specified MAC")
	}
	info.Printf("%sfound network device(%s) in domain config\n", id, mac)

	xmlOld, err = nicOld.Marshal()
	if err != nil {
		fail.Printf("%sfailed to marshal network device(%s): %s\n", id, mac, err.Error())
		return false, err
	}
	info.Printf("%smarshaled network device(%s) XML\n", id, mac)

	nicNew = nicOld
	nicNew.VLan = &vlan

	xmlNew, err = nicNew.Marshal()
	if err != nil {
		fail.Printf("%sfailed to marshal hostdev network device(%s) XML: %s\n", id, mac, err.Error())
		return false, err
	}
	info.Printf("%smarshaled hostdev network device(%s) XML\n", id, mac)

	err = detachNodeDeviceWithFlags(ctx, d, xmlOld, flags)
	if err != nil {
		return false, err
	}

	err = attachNodeDeviceWithFlags(ctx, d, xmlNew, flags)
	if err != nil {
		return false, err
	}

	nics, err = getDomainInterfaces(ctx, d)
	if err != nil {
		return false, err
	}

	for _, dev := range nics {
		if dev.MAC != nil {
			if dev.MAC.Address == mac {
				info.Printf("%schanged VLan PVID for network device\n", id)
				return true, nil
			}
		}
	}

	fail.Printf("%sno valide hostdev network device(%s) found in domain config\n", id, mac)
	return false, errors.New("no interfaces match specified MAC")
}

func getDomainInterfaceInfo(ctx context.Context, d *libvirt.Domain) ([]netInfo, error) {

	id := getReqIDFromContext(ctx)

	nics, err := getDomainInterfaces(ctx, d)
	if err != nil || len(nics) == 0 {
		return []netInfo{}, err
	}

	c, err := getConnectFromDomain(ctx, d)
	defer closeConnection(ctx, c)
	if err != nil || c == nil {
		return []netInfo{}, err
	}

	Nets := make([]netInfo, 0, len(nics))

	for _, dev := range nics {

		var vfPCIAddr, vfPCIName string
		var Net netInfo

		Net.Metadata, err = getDomainMetadata(ctx, d)
		if err != nil {
			continue
		}

		if dev.MAC != nil {
			Net.MAC = dev.MAC.Address
		} else {
			fail.Printf("%sdevice nas no MAC address\n", id)
			continue
		}

		ok := isInterfacePassthroughPCIDevice(ctx, dev)
		if !ok {
			continue
		}

		vfPCIAddr = fmt.Sprintf("%04x:%02x:%02x.%x",
			*dev.Source.Hostdev.PCI.Address.Domain,
			*dev.Source.Hostdev.PCI.Address.Bus,
			*dev.Source.Hostdev.PCI.Address.Slot,
			*dev.Source.Hostdev.PCI.Address.Function)
		vfPCIName = fmt.Sprintf("pci_%04x_%02x_%02x_%x",
			*dev.Source.Hostdev.PCI.Address.Domain,
			*dev.Source.Hostdev.PCI.Address.Bus,
			*dev.Source.Hostdev.PCI.Address.Slot,
			*dev.Source.Hostdev.PCI.Address.Function)

		// SR-IOV only one VLAN tag (PVID) is possible
		if dev.VLan != nil {
			if dev.VLan.Trunk == "" {
				for _, vid := range dev.VLan.Tags {
					if vid.NativeMode == "" {
						Net.PVID = fmt.Sprintf("%d", vid.ID)
						break
					}
				}
			} else {
				fail.Printf("%sdevice with MAC: %s has Trunk configuration\n", id, Net.MAC)
				continue
			}
		} else {
			fail.Printf("%sdevice with MAC: %s has no VLAN configuration\n", id, Net.MAC)
		}

		pfPCIName, pfPCIAddr, desc := getNodePFInfo(ctx, c, vfPCIName)
		if len(pfPCIName) != 0 && len(pfPCIAddr) != 0 {
			Net.PCI.VFName = vfPCIName
			Net.PCI.VFaddr = vfPCIAddr
			Net.PCI.PFName = pfPCIName
			Net.PCI.PFaddr = pfPCIAddr
			Net.Desc = desc

			vfPCIPath := getNodePCIDevicePath(ctx, c, vfPCIName)

			// find /sys/devices/pci0000:00/0000:00:03.2/0000:06:10.2/physfn/ -maxdepth 1 -type l -regex '.+/physfn/virtfn[0-9]+' -printf '%f%l\n'

			searchPath := path.Clean(fmt.Sprintf("%s/physfn/", vfPCIPath))
			files, err := ioutil.ReadDir(searchPath)
			if err != nil {
				fail.Printf("%sfailed to get VF name for %s: %s\n", id, vfPCIAddr, err.Error())
			}

			for _, f := range files {
				if !f.IsDir() {
					if f.Mode()&os.ModeSymlink != 0 {
						symlinkName := f.Name()
						if strings.HasPrefix(symlinkName, "virtfn") {

							symlinkPath := path.Clean(fmt.Sprintf("%s/%s", searchPath, symlinkName))
							filePath, err := os.Readlink(symlinkPath)
							if err != nil {
								continue
							}

							fileName := path.Base(filePath)
							if fileName != vfPCIAddr {
								continue
							}

							prettyVFName := strings.Replace(symlinkName, "virtfn", "vf", 1)
							info.Printf("%sVF for %s, %s\n", id, vfPCIAddr, prettyVFName)
							Net.VFName = prettyVFName
							break
						}
					}
				}
			}

			Net.PFName, err = findNodePFName(ctx, c, pfPCIName)
			if err != nil {
				continue
			}

			Net.Network, err = getNetworkNameForVFAddress(ctx, c, vfPCIAddr)
			if err != nil {
				continue
			}

		}

		Nets = append(Nets, Net)
	}

	return Nets, nil
}
