package main

import (
	"context"

	"github.com/libvirt/libvirt-go"
)

/*
func freeNetwork(ctx context.Context, n *libvirt.NodeDevice) {

	id := getReqIDFromContext(ctx)

	err := n.Free()
	if err != nil {
		fail.Printf("%sfailed to free network object: %s\n", id, err.Error())
	}

	info.Printf("%sfreed network object\n", id)
}
*/

func freeNetworks(ctx context.Context, nets []libvirt.Network) {

	id := getReqIDFromContext(ctx)

	for _, n := range nets {
		err := n.Free()
		if err != nil {
			fail.Printf("%sfailed to free network object: %s\n", id, err.Error())
		}
		info.Printf("%sfreed network object\n", id)
	}

}

func getNumOfNetworks(ctx context.Context, c *libvirt.Connect) (uint32, error) {

	id := getReqIDFromContext(ctx)

	count, err := c.NumOfNetworks()
	if err != nil {
		fail.Printf("%sfailed to get total number of active networks on hypervisor node: %s\n", id, err.Error())
		return 0, err
	}

	info.Printf("%sacquired total number of active networks on hypervisor node: %d\n", id, count)
	return uint32(count), nil
}

func listNetworks(ctx context.Context, c *libvirt.Connect) ([]libvirt.Network, error) {

	id := getReqIDFromContext(ctx)

	// here we acquire only active, persistent and autostarted networks
	flags := libvirt.CONNECT_LIST_NETWORKS_ACTIVE | libvirt.CONNECT_LIST_NETWORKS_PERSISTENT | libvirt.CONNECT_LIST_NETWORKS_AUTOSTART

	networks, err := c.ListAllNetworks(flags)
	if err != nil {
		fail.Printf("%sfailed to get list of networks: %s\n", id, err.Error())
		return []libvirt.Network{}, err
	}

	info.Printf("%sacquired list of networks\n", id)
	return networks, nil
}

func getNetworkName(ctx context.Context, n *libvirt.Network) (string, error) {

	id := getReqIDFromContext(ctx)

	name, err := n.GetName()
	if err != nil {
		fail.Printf("%sfailed to get network name: %s\n", id, err.Error())
		return "", err
	}

	info.Printf("%sacquired network name %s\n", id, name)
	return name, nil
}

func lookupNetworkByName(ctx context.Context, c *libvirt.Connect, name string) (*libvirt.Network, error) {

	id := getReqIDFromContext(ctx)

	network, err := c.LookupNetworkByName(name)
	if err != nil {
		fail.Printf("%sfailed to get network object for %s: %s\n", id, name, err.Error())
		return nil, err
	}

	info.Printf("%sacquired network object for %s\n", id, name)
	return network, nil
}
