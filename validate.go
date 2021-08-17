package main

import (
	"context"

	"github.com/libvirt/libvirt-go"
)

func validateCreateDomain(ctx context.Context, c *libvirt.Connect, uuid, name string, vCPU int, memory uint, storage, template, network, mac string) (bool, error) {

	ok, err := isUUIDValid(ctx, uuid)
	if err != nil || !ok {
		return false, err
	}

	ok, err = isDomainNameValidAndAvailable(ctx, c, name)
	if err != nil || !ok {
		return false, err
	}

	ok, err = isVCPUAvailable(ctx, c, vCPU)
	if err != nil || !ok {
		return false, err
	}

	ok, err = isMemoryAvailable(ctx, c, memory)
	if err != nil || !ok {
		return false, err
	}

	ok, err = isStorageAvailable(ctx, c, storage)
	if err != nil || !ok {
		return false, err
	}

	ok, err = isTemplateInsideStorageAvailable(ctx, c, storage, template)
	if err != nil || !ok {
		return false, err
	}

	ok, err = isNetworkVFAvailable(ctx, c, network)
	if err != nil || !ok {
		return false, err
	}

	ok, err = isMACvalid(ctx, mac)
	if err != nil || !ok {
		return false, err
	}

	return true, nil
}

func checkResources(ctx context.Context, c *libvirt.Connect, name string, vCPU int, memory uint, storagePool, network string) (bool, error) {

	ok, err := isDomainNameValidAndAvailable(ctx, c, name)
	if err != nil || !ok {
		return false, err
	}

	ok, err = isVCPUAvailable(ctx, c, vCPU)
	if err != nil || !ok {
		return false, err
	}

	ok, err = isMemoryAvailable(ctx, c, memory)
	if err != nil || !ok {
		return false, err
	}

	ok, err = isStorageAvailable(ctx, c, storagePool)
	if err != nil || !ok {
		return false, err
	}

	ok, err = isNetworkVFAvailable(ctx, c, network)
	if err != nil || !ok {
		return false, err
	}

	return true, nil
}
