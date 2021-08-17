package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/libvirt/libvirt-go"
	"github.com/libvirt/libvirt-go-xml"
)

/*
func getNodeDeviceParent(ctx context.Context, c *libvirt.Connect, dev *libvirt.NodeDevice) (string, error) {

	id := getReqIDFromContext(ctx)

	parent, err := dev.GetParent()
	if err != nil {
		fail.Printf("%sfailed to get parent for node network device: %s\n", id, err.Error())
		return "", err
	}

	info.Printf("%snode network device parent %s\n", id, parent)
	return parent, nil
}
*/

/*
func getNodeDeviceName(ctx context.Context, c *libvirt.Connect, dev *libvirt.NodeDevice) (string, error) {

	id := getReqIDFromContext(ctx)

	name, err := dev.GetName()
	if err != nil {
		fail.Printf("%sfailed to get node network device name: %s\n", id, err.Error())
		return "", err
	}

	info.Printf("%snode network device name: %s", id, name)
	return name, nil
}
*/

func listNodeNetworkDevices(ctx context.Context, c *libvirt.Connect) ([]libvirt.NodeDevice, error) {

	id := getReqIDFromContext(ctx)

	devs, err := c.ListAllNodeDevices(libvirt.CONNECT_LIST_NODE_DEVICES_CAP_NET)
	if err != nil {
		fail.Printf("%sfailed to get list of node network devices: %s\n", id, err.Error())
		return []libvirt.NodeDevice{}, err
	}

	if len(devs) == 0 {
		fail.Printf("%sfailed to get list of node network devices: %s\n", id, errors.New("network array has zero size"))
		return []libvirt.NodeDevice{}, errors.New("network array has zero size")
	}

	info.Printf("%sacquired list of node network devices\n", id)
	return devs, nil
}

func detachNodeDeviceWithFlags(ctx context.Context, d *libvirt.Domain, xml string, flags libvirt.DomainDeviceModifyFlags) error {

	id := getReqIDFromContext(ctx)

	err := d.DetachDeviceFlags(xml, flags)
	if err != nil {
		fail.Printf("%sfailed to remove SR-IOV network device from domain config: %s\n", id, err.Error())
		return err
	}

	time.Sleep(3 * time.Second)

	info.Printf("%sremoved SR-IOV network device from domain config:\n", id)
	return nil
}

func attachNodeDeviceWithFlags(ctx context.Context, d *libvirt.Domain, xml string, flags libvirt.DomainDeviceModifyFlags) error {

	id := getReqIDFromContext(ctx)

	err := d.AttachDeviceFlags(xml, flags)
	if err != nil {
		fail.Printf("%sfailed to re-attach SR-IOV network device from domain config: %s\n", id, err.Error())
		return err
	}

	time.Sleep(3 * time.Second)

	info.Printf("%sre-attached SR-IOV network device from domain config:\n", id)
	return nil
}

func lookupNodeDeviceByName(ctx context.Context, c *libvirt.Connect, devID string) (*libvirt.NodeDevice, error) {

	id := getReqIDFromContext(ctx)

	dev, err := c.LookupDeviceByName(devID)
	if err != nil || dev == nil {
		fail.Printf("%sfailed to find device %s: %s\n", id, devID, err.Error())
		return nil, err
	}

	info.Printf("%sfound device %s\n", id, devID)
	return dev, nil
}

func freeNodeDevice(ctx context.Context, n *libvirt.NodeDevice) {

	id := getReqIDFromContext(ctx)

	err := n.Free()
	if err != nil {
		fail.Printf("%sfailed to free node device object: %s\n", id, err.Error())
	}

	info.Printf("%sfreed node device object\n", id)
}

func freeNodeDevices(ctx context.Context, devs []libvirt.NodeDevice) {

	id := getReqIDFromContext(ctx)

	for _, dev := range devs {
		err := dev.Free()
		if err != nil {
			fail.Printf("%sfailed to free node device object: %s\n", id, err.Error())
		}
		info.Printf("%sfreed node device object\n", id)
	}

}

func getNodePCIDevicePath(ctx context.Context, c *libvirt.Connect, devName string) string {

	id := getReqIDFromContext(ctx)

	var err error

	dev, err := lookupNodeDeviceByName(ctx, c, devName)
	defer freeNodeDevice(ctx, dev)
	if err != nil || dev == nil {
		return ""
	}

	xml, err := dev.GetXMLDesc(0)
	if err != nil {
		fail.Printf("%sfailed to get device %s XML: %s\n", id, devName, err.Error())
		return ""
	}
	info.Printf("%sacquired device %s XML\n", id, devName)

	nodeDev := &libvirtxml.NodeDevice{}
	err = nodeDev.Unmarshal(xml)
	if err != nil {
		fail.Printf("%sfailed to unmarshal device %s XML: %s\n", id, devName, err.Error())
		return ""
	}
	info.Printf("%sunmarshaled device %s XML\n", id, devName)

	info.Printf("%sdevice %s PCI path %s\n", id, devName, nodeDev.Path)
	return strings.TrimSpace(nodeDev.Path)
}

func getDomainBlockDeviceNamesOrPaths(ctx context.Context, d *libvirt.Domain, getPath bool) ([]string, error) {

	id := getReqIDFromContext(ctx)

	var disks, paths []string

	xml, err := d.GetXMLDesc(0)
	if err != nil {
		fail.Printf("%sfailed to get Domain XML: %s\n", id, err.Error())
		return []string{}, err
	}
	info.Printf("%sacquired Domain XML\n", id)

	domCfg := &libvirtxml.Domain{}
	err = domCfg.Unmarshal(xml)
	if err != nil {
		fail.Printf("%sfailed to parse Domain XML: %s\n", id, err.Error())
		return []string{}, err
	}
	info.Printf("%sparsed Domain XML\n", id)

	if domCfg.Devices != nil {
		for _, dev := range domCfg.Devices.Disks {
			if dev.Device == "disk" {
				if dev.Target != nil {
					disks = append(disks, dev.Target.Dev)
					info.Printf("%sfound disk %s in Domain XML\n", id, dev.Target.Dev)
				}
				if dev.Source != nil {
					if dev.Source.File != nil {
						paths = append(paths, dev.Source.File.File)
						info.Printf("%sfound path %s in Domain XML\n", id, dev.Source.File.File)
					}
				}
			}
		}
	}

	info.Printf("%sacquired block device list from domain XML\n", id)
	if getPath {
		return paths, nil
	}

	return disks, nil
}
