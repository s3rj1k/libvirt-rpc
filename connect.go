package main

import (
	"context"
	"strings"

	"github.com/libvirt/libvirt-go"
)

func getConnectFromDomain(ctx context.Context, d *libvirt.Domain) (*libvirt.Connect, error) {

	id := getReqIDFromContext(ctx)

	c, err := d.DomainGetConnect()
	if err != nil || c == nil {
		fail.Printf("%sfailed to get active hypervisor connection object: %s\n", id, err.Error())
		return nil, err
	}

	info.Printf("%sacquired object for active hypervisor connection\n", id)
	return c, nil
}

func openConnection(ctx context.Context, flag string) (*libvirt.Connect, error) {

	id := getReqIDFromContext(ctx)

	var c *libvirt.Connect
	var err error

	uri := []string{"qemu:///system"}

	switch flag {
	case "ro":
		c, err = libvirt.NewConnectReadOnly(strings.Join(uri, ""))
	case "rw":
		c, err = libvirt.NewConnect(strings.Join(uri, ""))
	default:
		c, err = libvirt.NewConnectReadOnly(strings.Join(uri, ""))
	}

	if err != nil {
		fail.Printf("%slocal hypervisor connection failed: %s\n", id, err.Error())
		return &libvirt.Connect{}, err
	}

	info.Printf("%sconnected to local hypervisor\n", id)
	return c, nil
}

func closeConnection(ctx context.Context, c *libvirt.Connect) {

	id := getReqIDFromContext(ctx)

	var s string

	if c != nil {

		host, err := getNodeHostname(ctx, c)
		if err != nil {
			host = unknown
		}

		r, err := c.Close()

		if r == -1 {
			s = "error"
		} else if r == 0 {
			s = "success"
		} else if r > 0 {
			s = "has references"
		} else {
			s = "error"
		}

		info.Printf("%sconnection to %s closed: %s\n", id, host, s)

		if err != nil {
			fail.Printf("%sfailed to close connection: %s\n", id, err.Error())
		}

	} else {
		info.Printf("%sno available connection to close\n", id)
	}

}
