package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/libvirt/libvirt-go"
	"github.com/libvirt/libvirt-go-xml"
)

type domainMemoryStats struct {
	SwapIn     uint64 // KiB
	SwapOut    uint64 // KiB
	MajorFault uint64
	MinorFault uint64
	Unused     uint64 // KiB
	Available  uint64 // KiB
	Actual     uint64 // KiB
	Usable     uint64 // KiB
	LastUpdate uint64
	Rss        uint64 // KiB
}

func getDomainMemoryStats(ctx context.Context, d *libvirt.Domain) (domainMemoryStats, error) {

	id := getReqIDFromContext(ctx)

	var m domainMemoryStats

	out, err := d.MemoryStats(uint32(libvirt.DOMAIN_MEMORY_STAT_NR), 0)
	if err != nil {
		fail.Printf("%sfailed to get memory stats for domain: %s\n", id, err.Error())
		return domainMemoryStats{}, err
	}

	for _, el := range out {
		tag := libvirt.DomainMemoryStatTags(el.Tag)

		switch tag {
		case libvirt.DOMAIN_MEMORY_STAT_SWAP_IN:
			m.SwapIn = el.Val
		case libvirt.DOMAIN_MEMORY_STAT_SWAP_OUT:
			m.SwapOut = el.Val
		case libvirt.DOMAIN_MEMORY_STAT_MAJOR_FAULT:
			m.MajorFault = el.Val
		case libvirt.DOMAIN_MEMORY_STAT_MINOR_FAULT:
			m.MinorFault = el.Val
		case libvirt.DOMAIN_MEMORY_STAT_UNUSED:
			m.Unused = el.Val
		case libvirt.DOMAIN_MEMORY_STAT_AVAILABLE:
			m.Available = el.Val
		case libvirt.DOMAIN_MEMORY_STAT_ACTUAL_BALLOON:
			m.Actual = el.Val
		case libvirt.DOMAIN_MEMORY_STAT_RSS:
			m.Rss = el.Val
		case libvirt.DOMAIN_MEMORY_STAT_USABLE:
			m.Usable = el.Val
		case libvirt.DOMAIN_MEMORY_STAT_LAST_UPDATE:
			m.LastUpdate = el.Val
		}
	}

	info.Printf("%sacquired memory stats for domain\n", id)
	return m, nil
}

func setDomainCurrentMemory(ctx context.Context, d *libvirt.Domain, mem uint64) error {

	id := getReqIDFromContext(ctx)

	flags := libvirt.DOMAIN_MEM_CURRENT

	if ok := isDomainPersistent(ctx, d); ok {
		flags = flags | libvirt.DOMAIN_MEM_CONFIG
	}

	if ok := isDomainActive(ctx, d); ok {
		flags = flags | libvirt.DOMAIN_MEM_LIVE
	}

	err := d.SetMemoryFlags(mem, flags)
	if err != nil {
		fail.Printf("%sfailed to set current available memory for domain: %s\n", id, err.Error())
		return err
	}

	info.Printf("%scurrent available memory for domain set to %d KiB\n", id, mem)
	return nil
}

func setDomainMemoryStatsPeriod(ctx context.Context, d *libvirt.Domain, period int) error {

	id := getReqIDFromContext(ctx)

	flags := libvirt.DOMAIN_MEM_CURRENT

	if ok := isDomainPersistent(ctx, d); ok {
		flags = flags | libvirt.DOMAIN_MEM_CONFIG
	}

	if ok := isDomainActive(ctx, d); ok {
		flags = flags | libvirt.DOMAIN_MEM_LIVE
	}

	err := d.SetMemoryStatsPeriod(period, flags)
	if err != nil {
		fail.Printf("%sfailed to set period of memory stats collection for domain: %s\n", id, err.Error())
		return err
	}

	info.Printf("%speriod of memory stats collection for domain set to %d seconds\n", id, period)
	return nil
}

func getDomainMemoryStatsPeriod(ctx context.Context, d *libvirt.Domain) (uint, error) {

	id := getReqIDFromContext(ctx)

	xmlDoc, err := d.GetXMLDesc(0)
	if err != nil {
		fail.Printf("%sfailed to get memory stats period for domain: %s\n", id, err.Error())
		return 0, err
	}
	info.Printf("%sacquired hypervisor XML\n", id)

	domCfg := &libvirtxml.Domain{}
	err = domCfg.Unmarshal(xmlDoc)
	if err != nil {
		fail.Printf("%sfailed to get memory stats period for domain: %s\n", id, err.Error())
		return 0, err
	}

	if domCfg.Devices == nil {
		fail.Printf("%sfailed to get memory stats period for domain: %s\n", id, errors.New("invalid domain XML"))
		return 0, errors.New("invalid domain XML")
	}

	if domCfg.Devices.MemBalloon == nil {
		fail.Printf("%sfailed to get memory stats period for domain: %s\n", id, errors.New("invalid domain XML"))
		return 0, errors.New("invalid domain XML")
	}

	if domCfg.Devices.MemBalloon.Stats == nil {
		fail.Printf("%sfailed to get memory stats period for domain: %s\n", id, errors.New("invalid domain XML"))
		return 0, errors.New("invalid domain XML")
	}

	period := domCfg.Devices.MemBalloon.Stats.Period

	info.Printf("%speriod of memory stats collection for domain set to %d seconds\n", id, period)
	return period, nil
}

func setDomainMaxMemory(ctx context.Context, d *libvirt.Domain, mem uint64) error {

	id := getReqIDFromContext(ctx)

	flags := libvirt.DOMAIN_MEM_CONFIG | libvirt.DOMAIN_MEM_MAXIMUM

	err := d.SetMemoryFlags(mem, flags)
	if err != nil {
		fail.Printf("%sfailed to set maximum available memory for domain: %s\n", id, err.Error())
		return err
	}

	info.Printf("%smaximum memory for domain set to %d KiB\n", id, mem)
	return nil
}

// func getNodeMemoryStats(ctx context.Context, c *libvirt.Connect) (*libvirt.NodeMemoryStats, error) {

// 	id := getReqIDFromContext(ctx)

// 	memStats, err := c.GetMemoryStats(-1, 0)
// 	if err != nil {
// 		fail.Printf("%sfailed to get memory stats on hypervisor node: %s\n", id, err.Error())
// 		return nil, err
// 	}

// 	if memStats == nil {
// 		fail.Printf("%sfailed to get memory stats on hypervisor node: %s\n", id, errors.New("memory stats can not be empty"))
// 		return nil, errors.New("memory stats can not be empty")
// 	}

// 	info.Printf("%sacquired memory stats on hypervisor node\n", id)
// 	return memStats, nil
// }

func getNodeMemoryStats(ctx context.Context) (nodeMemoryStats, error) {

	id := getReqIDFromContext(ctx)

	var err error

	data, err := ioutil.ReadFile("/proc/meminfo")
	if err != nil {
		fail.Printf("%sfailed to parse /proc/meminfo: %s\n", id, err.Error())
		return nodeMemoryStats{}, fmt.Errorf("failed to parse /proc/meminfo: %s", err.Error())
	}

	var memInfo nodeMemoryStats

	lines := strings.Split(string(data), "\n")

	for _, line := range lines {

		fields := strings.SplitN(line, ":", 2)
		if len(fields) < 2 {
			continue
		}

		name := fields[0]
		valFields := strings.Fields(fields[1])

		val, err := strconv.ParseUint(valFields[0], 10, 64)
		if err != nil {
			continue
		}

		switch strings.ToUpper(name) {
		case strings.ToUpper("MemTotal"):
			memInfo.Total = val
		case strings.ToUpper("MemAvailable"):
			memInfo.Available = val
		case strings.ToUpper("MemUsed"):
			memInfo.Used = val
		case strings.ToUpper("MemFree"):
			memInfo.Free = val
		case strings.ToUpper("Cached"):
			memInfo.Cached = val
		case strings.ToUpper("Buffers"):
			memInfo.Buffers = val
		case strings.ToUpper("SwapTotal"):
			memInfo.SwapTotal = val
		case strings.ToUpper("SwapFree"):
			memInfo.SwapFree = val
		case strings.ToUpper("SwapCached"):
			memInfo.SwapCached = val
		}

	}

	/* if kb_main_available is greater than kb_main_total or our calculation of
	   mem_used overflows, that's symptomatic of running within a lxc container
		 where such values will be dramatically distorted over those of the host.

		 https://gitlab.com/procps-ng/procps/blob/master/proc/sysinfo.c#L787
	*/
	if memInfo.Available > memInfo.Total {
		memInfo.Available = memInfo.Free
	}

	memInfo.Used = memInfo.Total - memInfo.Free - memInfo.Cached - memInfo.Buffers

	if memInfo.Used < 0 {
		memInfo.Used = memInfo.Total - memInfo.Free
	}

	info.Printf("%sacquired memory stats on hypervisor node\n", id)
	return memInfo, nil
}

func isMemoryAvailable(ctx context.Context, c *libvirt.Connect, memory uint) (bool, error) {

	id := getReqIDFromContext(ctx)

	maxMemory := 2 * memory

	nodeMemStats, err := getNodeMemoryStats(ctx)
	if err != nil {
		return false, err
	}

	if memory == 0 {
		fail.Printf("%smemory can not be 0\n", id)
		return false, fmt.Errorf("memory can not be 0")
	}

	if memory < 256*1024 {
		fail.Printf("%smemory can not be lesser that 256 MB\n", id)
		return false, fmt.Errorf("memory can not be lesser that 256 MB")
	}

	if uint64(maxMemory) > nodeMemStats.Available {
		fail.Printf("%samount of maximum memory for domain: %d KiB is greater than available memory on hypervisor: %d KiB\n", id, maxMemory, nodeMemStats.Available)
		return false, fmt.Errorf("amount of maximum memory for domain: %d KiB is greater than available memory on hypervisor: %d KiB", maxMemory, nodeMemStats.Free)
	}

	info.Printf("%sMemory: %d available\n", id, nodeMemStats.Available)
	return true, nil
}
