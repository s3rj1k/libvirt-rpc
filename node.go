package main

import (
	"context"
	"errors"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/libvirt/libvirt-go"
)

func getNodeHostname(ctx context.Context, c *libvirt.Connect) (string, error) {

	id := getReqIDFromContext(ctx)

	var err error

	host, err := c.GetHostname()
	if err != nil {
		fail.Printf("%sfailed to get hypervisor host: %s\n", id, err.Error())
		return "", err
	}

	info.Printf("%shypervisor hostname: %s", id, host)
	return host, nil
}

func getNodeUptime(ctx context.Context, c *libvirt.Connect) (uint64, error) {

	id := getReqIDFromContext(ctx)

	var err error

	b, err := ioutil.ReadFile("/proc/uptime")
	if err != nil {
		return 0, err
	}

	uptimeSlice := strings.Split(strings.TrimSpace(string(b[:])), " ")
	if len(uptimeSlice) != 2 {
		fail.Printf("%sfailed to parse /proc/uptime: %s\n", id, err.Error())
		return 0, errors.New("unknown /proc/uptime format")
	}

	uptime, err := strconv.ParseFloat(uptimeSlice[0], 64)
	if err != nil {
		fail.Printf("%sfailed to convert /proc/uptime to float64: %s\n", id, err.Error())
		return 0, err
	}

	return uint64(uptime * 1000 * 1000 * 1000), nil
}

func getNodeLibVersion(ctx context.Context, c *libvirt.Connect) (uint32, error) {

	id := getReqIDFromContext(ctx)

	version, err := c.GetLibVersion()
	if err != nil {
		fail.Printf("%sfailed to get libvirt version on hypervisor node: %s\n", id, err.Error())
		return 0, err
	}
	info.Printf("%sacquired libvirt version on hypervisor node: %d\n", id, version)
	return version, nil
}

func getNodeInfo(ctx context.Context, c *libvirt.Connect) (*libvirt.NodeInfo, error) {

	id := getReqIDFromContext(ctx)

	nodeInfo, err := c.GetNodeInfo()
	if err != nil {
		fail.Printf("%sfailed to get hypervisor node info: %s\n", id, err.Error())
		return nil, err
	}

	if nodeInfo == nil {
		fail.Printf("%sfailed to get hypervisor node info: %s\n", id, errors.New("node info can not be empty"))
		return nil, errors.New("node info can not be empty")
	}

	info.Printf("%sacquired hypervisor node info\n", id)
	return nodeInfo, nil
}
