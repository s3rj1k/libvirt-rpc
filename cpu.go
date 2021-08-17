package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/libvirt/libvirt-go"
)

func getDomainMaxVCPUs(ctx context.Context, d *libvirt.Domain) uint64 {

	id := getReqIDFromContext(ctx)

	i, err := d.GetMaxVcpus()
	if err != nil {
		fail.Printf("%sfailed to get maximum vCPUs for domain: %s\n", id, err.Error())
		return 0
	}

	info.Printf("%sacquired maximum vCPUs %d for domain\n", id, i)
	return uint64(i)
}

func getDomainCurrentVCPUs(ctx context.Context, d *libvirt.Domain) uint64 {

	id := getReqIDFromContext(ctx)

	i, err := d.GetVcpusFlags(libvirt.DOMAIN_VCPU_CURRENT)
	if err != nil {
		fail.Printf("%sfailed to get current vCPUs for domain: %s\n", id, err.Error())
		return 0
	}

	info.Printf("%sacquired current vCPUs %d for domain\n", id, i)
	return uint64(i)
}

func setDomainCurrentVCPUs(ctx context.Context, d *libvirt.Domain, vCPUNum uint) error {

	id := getReqIDFromContext(ctx)

	flags := libvirt.DOMAIN_VCPU_CURRENT

	if ok := isDomainPersistent(ctx, d); ok {
		flags = flags | libvirt.DOMAIN_VCPU_CONFIG
	}

	if ok := isDomainActive(ctx, d); ok {
		flags = flags | libvirt.DOMAIN_VCPU_LIVE
	}

	err := d.SetVcpusFlags(vCPUNum, flags)
	if err != nil {
		fail.Printf("%sfailed to set current online vCPUs for domain: %s\n", id, err.Error())
		return err
	}

	info.Printf("%scurrent online vCPUs for domain set to %d\n", id, vCPUNum)
	return nil
}

func setDomainMaxVCPUs(ctx context.Context, d *libvirt.Domain, vCPUNum uint) error {

	id := getReqIDFromContext(ctx)

	flags := libvirt.DOMAIN_VCPU_CONFIG | libvirt.DOMAIN_VCPU_MAXIMUM | libvirt.DOMAIN_VCPU_HOTPLUGGABLE

	err := d.SetVcpusFlags(vCPUNum, flags)
	if err != nil {
		fail.Printf("%sfailed to set maximum vCPUs value for domain: %s\n", id, err.Error())
		return err
	}

	info.Printf("%smaximum vCPUs value for domain set to %d\n", id, vCPUNum)
	return nil
}

func getNodeCPUStats(ctx context.Context, c *libvirt.Connect) (*libvirt.NodeCPUStats, error) {

	id := getReqIDFromContext(ctx)

	cpuStats, err := c.GetCPUStats(-1, 0)
	if err != nil {
		fail.Printf("%sfailed to get CPU stats on hypervisor node: %s\n", id, err.Error())
		return nil, err
	}

	if cpuStats == nil {
		fail.Printf("%sfailed to get CPU stats on hypervisor node: %s\n", id, errors.New("CPU stats can not be empty"))
		return nil, errors.New("CPU stats can not be empty")
	}

	info.Printf("%sacquired CPU stats on hypervisor node\n", id)
	return cpuStats, nil
}

func getNumOfAssignedNodeVCPUs(ctx context.Context, c *libvirt.Connect) (uint32, error) {

	var count int32

	id := getReqIDFromContext(ctx)

	domains, err := listAllDomainsWithFlags(ctx, c, libvirt.ConnectListAllDomainsFlags(0))
	defer freeDomains(ctx, domains)
	if err != nil {
		return 0, err
	}

	for _, d := range domains {
		num, err := d.GetVcpusFlags(libvirt.DOMAIN_VCPU_CURRENT)
		if err != nil {
			continue
		}
		count = count + num
	}

	info.Printf("%sacquired number of globally assigned vCPUs: %d\n", id, count)
	return uint32(count), nil
}

func isVCPUAvailable(ctx context.Context, c *libvirt.Connect, vCPU int) (bool, error) {

	id := getReqIDFromContext(ctx)

	nodeInfo, err := getNodeInfo(ctx, c)
	if err != nil {
		return false, err
	}

	if vCPU == 0 {
		fail.Printf("%svCPU can not be 0\n", id)
		return false, fmt.Errorf("vCPU can not be 0")
	}

	if uint(vCPU) > nodeInfo.Cpus {
		fail.Printf("%samount of vCPUs: %d are greater than physically available hypervisor cores: %d\n", id, vCPU, nodeInfo.Cpus)
		return false, fmt.Errorf("amount of vCPUs: %d are greater than physically available hypervisor cores: %d", vCPU, nodeInfo.Cpus)
	}

	info.Printf("%svCPU(s): %d available\n", id, vCPU)
	return true, nil
}
