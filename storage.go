package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/libvirt/libvirt-go"
	"github.com/libvirt/libvirt-go-xml"
)

func listStorgePools(ctx context.Context, c *libvirt.Connect, flags libvirt.ConnectListAllStoragePoolsFlags) ([]libvirt.StoragePool, error) {

	id := getReqIDFromContext(ctx)

	pools, err := c.ListAllStoragePools(flags)
	if err != nil {
		fail.Printf("%sfailed to get list of storage pools: %s\n", id, err.Error())
		return []libvirt.StoragePool{}, err
	}

	info.Printf("%sacquired list of storage pools\n", id)
	return pools, nil
}

func listAllStorgeVolumesInPool(ctx context.Context, p *libvirt.StoragePool) ([]libvirt.StorageVol, error) {

	id := getReqIDFromContext(ctx)

	volumes, err := p.ListAllStorageVolumes(0)
	if err != nil {
		fail.Printf("%sfailed to get storage volumes on pool: %s\n", id, err.Error())
		return []libvirt.StorageVol{}, err
	}

	info.Printf("%sacquired list of volumes in storage pool\n", id)
	return volumes, nil
}

func countNumOfStorageVolumesInPool(ctx context.Context, p *libvirt.StoragePool) (int, error) {

	id := getReqIDFromContext(ctx)

	count, err := p.NumOfStorageVolumes()
	if err != nil {
		fail.Printf("%sfailed to get number of volumes in storage pool: %s\n", id, err.Error())
		return 0, err
	}

	info.Printf("%sacquired number(%d) of volumes in pool\n", id, count)
	return count, nil
}

func refreshPool(ctx context.Context, p *libvirt.StoragePool) error {

	id := getReqIDFromContext(ctx)

	err := p.Refresh(0)
	if err != nil {
		fail.Printf("%sfailed to refresh storage pool: %s\n", id, err.Error())
		return err
	}

	info.Printf("%srefreshed storage pool\n", id)
	return nil
}

func refreshAllStorgePools(ctx context.Context, c *libvirt.Connect) error {

	id := getReqIDFromContext(ctx)

	// here we acquire only directory based pools
	pools, err := listStorgePools(ctx, c, libvirt.CONNECT_LIST_STORAGE_POOLS_DIR|libvirt.CONNECT_LIST_STORAGE_POOLS_ACTIVE)
	defer freePools(ctx, pools)

	if err != nil {
		fail.Printf("%sfailed to get storage pool objects: %s\n", id, err.Error())
		return err
	}

	info.Printf("%sacquired storage pool(s) object(s)\n", id)

	for _, pool := range pools {
		err = refreshPool(ctx, &pool)
		if err != nil {
			continue
		}
	}

	return nil
}

func getPoolName(ctx context.Context, p *libvirt.StoragePool) (string, error) {

	id := getReqIDFromContext(ctx)

	name, err := p.GetName()
	if err != nil {
		fail.Printf("%sfailed to get name of storage pool: %s\n", id, err.Error())
		return "", err
	}

	info.Printf("%sacquired storage pool named %s\n", id, name)
	return name, nil
}

func listPoolVolumes(ctx context.Context, p *libvirt.StoragePool) ([]string, error) {

	id := getReqIDFromContext(ctx)

	volumes, err := p.ListStorageVolumes()
	if err != nil {
		fail.Printf("%sfailed to get storage pool list of volumes: %s\n", id, err.Error())
		return []string{}, err
	}

	info.Printf("%sacquired storage pool list of volumes\n", id)
	return volumes, nil
}

func lookupStorageVolByPath(ctx context.Context, c *libvirt.Connect, path string) (*libvirt.StorageVol, error) {

	id := getReqIDFromContext(ctx)

	vol, err := c.LookupStorageVolByPath(path)
	if err != nil {
		fail.Printf("%sfailed to get storage volume object for %s: %s\n", id, path, err.Error())
		return nil, err
	}

	info.Printf("%sacquired storage volume object for: %s\n", id, path)
	return vol, nil
}

func lookupPoolByVolume(ctx context.Context, vol *libvirt.StorageVol) (*libvirt.StoragePool, error) {

	id := getReqIDFromContext(ctx)

	pool, err := vol.LookupPoolByVolume()
	if err != nil {
		fail.Printf("%sfailed to get storage pool object: %s\n", id, err.Error())
		return nil, err
	}

	info.Printf("%sacquired storage pool object\n", id)
	return pool, nil
}

func lookupPoolByName(ctx context.Context, c *libvirt.Connect, name string) (*libvirt.StoragePool, error) {

	id := getReqIDFromContext(ctx)

	pool, err := c.LookupStoragePoolByName(name)
	if err != nil {
		fail.Printf("%sfailed to get storage pool object: %s\n", id, err.Error())
		return nil, err
	}

	info.Printf("%sacquired storage pool object\n", id)
	return pool, nil
}

func getVolumePath(ctx context.Context, v *libvirt.StorageVol) (string, error) {

	id := getReqIDFromContext(ctx)

	path, err := v.GetPath()
	if err != nil {
		fail.Printf("%sfailed to get storage volume path: %s\n", id, err.Error())
		return "", err
	}

	info.Printf("%sstorage volume path: %s\n", id, path)
	return path, nil
}

func deletePoolVolume(ctx context.Context, v *libvirt.StorageVol, flags libvirt.StorageVolDeleteFlags) error {

	id := getReqIDFromContext(ctx)

	err := v.Delete(flags)
	if err != nil {
		fail.Printf("%sfailed to delete storage volume: %s\n", id, err.Error())
		return err
	}

	info.Printf("%sdestroyed storage volume\n", id)
	return nil
}

func cloneVolumeByPath(ctx context.Context, c *libvirt.Connect, storage, leftImageName, rightImageName string) error {

	id := getReqIDFromContext(ctx)

	pool, err := lookupPoolByName(ctx, c, storage)
	if err != nil {
		return err
	}
	defer freePool(ctx, pool)

	poolPath, err := getPoolPath(ctx, pool)
	if err != nil {
		return err
	}

	volPath := filepath.Clean(fmt.Sprintf("%s/%s", poolPath, rightImageName))

	cloneVol, err := lookupStorageVolByName(ctx, c, pool, leftImageName)
	if err != nil {
		return err
	}

	defer freeVolume(ctx, cloneVol)

	xml, err := cloneVol.GetXMLDesc(0)
	if err != nil {
		fail.Printf("%sfailed to get XML for volume object: %s\n", id, err.Error())
		return err
	}
	info.Printf("%sacquired storage volume XML\n", id)

	volCfg := &libvirtxml.StorageVolume{}

	err = volCfg.Unmarshal(xml)
	if err != nil {
		fail.Printf("%sfailed to unmarshal XML to structure: %s\n", id, err.Error())
		return err
	}
	info.Printf("%sunmarshaled storage volume XML\n", id)

	if volCfg.BackingStore != nil {
		return fmt.Errorf("not cloning, volume has backing strore")
	}

	volCfg.Name = rightImageName
	volCfg.Key = volPath

	volCfg.Allocation = nil
	volCfg.Capacity = nil
	volCfg.Physical = nil
	volCfg.BackingStore = nil

	if volCfg.Target == nil {
		return fmt.Errorf("not cloning, volume has no target description")
	}

	volCfg.Target.Timestamps = nil
	volCfg.Target.Path = volPath

	xml, err = volCfg.Marshal()
	if err != nil {
		fail.Printf("%sfailed to marshal XML from structure: %s\n", id, err.Error())
		return err
	}
	info.Printf("%smarshaled storage volume XML\n", id)

	newVol, err := pool.StorageVolCreateXMLFrom(xml, cloneVol, 0)
	if err != nil {
		fail.Printf("%sfailed to clone volume using XML config: %s\n", id, err.Error())
		return err
	}
	info.Printf("%scloned storage volume from %s/%s to path: %s\n", id, storage, leftImageName, volPath)
	defer freeVolume(ctx, newVol)

	return nil
}

func getPoolPath(ctx context.Context, p *libvirt.StoragePool) (string, error) {

	id := getReqIDFromContext(ctx)

	xml, err := p.GetXMLDesc(libvirt.StorageXMLFlags(0))
	if err != nil {
		fail.Printf("%sfailed to get XML of storage pool: %s\n", id, err.Error())
		return "", err
	}
	info.Printf("%sacquired storage pool XML\n", id)

	poolCfg := &libvirtxml.StoragePool{}
	err = poolCfg.Unmarshal(xml)
	if err != nil {
		fail.Printf("%sfailed to unmarshal storage pool XML: %s\n", id, err.Error())
		return "", err
	}
	info.Printf("%sunmarshaled storage pool XML\n", id)

	if poolCfg.Target != nil {
		info.Printf("%sacquired storage pool path\n", id)
		return poolCfg.Target.Path, nil
	}

	fail.Printf("%sfailed to get storage pool path: %s\n", id, errors.New("empty target in storage pool XML"))
	return "", errors.New("empty target in storage pool XML")
}

func getPoolInfo(ctx context.Context, p *libvirt.StoragePool) (*libvirt.StoragePoolInfo, error) {

	id := getReqIDFromContext(ctx)

	poolInfo, err := p.GetInfo()
	if err != nil {
		fail.Printf("%sfailed to get info for storage pool: %s\n", id, err.Error())
		return nil, err
	}

	info.Printf("%sacquired storage pool info\n", id)
	return poolInfo, nil
}

func isPoolAutostarted(ctx context.Context, p *libvirt.StoragePool) (bool, error) {

	id := getReqIDFromContext(ctx)

	isAutostarted, err := p.GetAutostart()
	if err != nil {
		fail.Printf("%sfailed to get autostart status for storage pool: %s\n", id, err.Error())
		return false, err
	}

	info.Printf("%sacquired autostart status(%t) for pool\n", id, isAutostarted)
	return isAutostarted, nil
}

func isPoolActive(ctx context.Context, p *libvirt.StoragePool) (bool, error) {

	id := getReqIDFromContext(ctx)

	isActive, err := p.IsActive()
	if err != nil {
		fail.Printf("%sfailed to get active status for storage pool: %s\n", id, err.Error())
		return false, err
	}

	info.Printf("%sacquired active status(%t) for pool\n", id, isActive)
	return isActive, nil
}

func isPoolPersistent(ctx context.Context, p *libvirt.StoragePool) (bool, error) {

	id := getReqIDFromContext(ctx)

	isPersistent, err := p.IsPersistent()
	if err != nil {
		fail.Printf("%sfailed to get persistent status for storage pool: %s\n", id, err.Error())
		return false, err
	}

	info.Printf("%sacquired persistent status(%t) for pool\n", id, isPersistent)
	return isPersistent, nil
}

func freeVolumes(ctx context.Context, v []libvirt.StorageVol) {

	id := getReqIDFromContext(ctx)

	for _, e := range v {
		err := e.Free()
		if err != nil {
			fail.Printf("%sfailed to free storage volume object: %s\n", id, err.Error())
		} else {
			info.Printf("%sfreed storage volume object\n", id)
		}
	}
}

func freeVolume(ctx context.Context, v *libvirt.StorageVol) {

	id := getReqIDFromContext(ctx)

	err := v.Free()
	if err != nil {
		fail.Printf("%sfailed to free storage volume object: %s\n", id, err.Error())
	} else {
		info.Printf("%sfreed domain volume object\n", id)
	}
}

func freePools(ctx context.Context, p []libvirt.StoragePool) {

	id := getReqIDFromContext(ctx)

	for _, e := range p {
		err := e.Free()
		if err != nil {
			fail.Printf("%sfailed to free storage pool object: %s\n", id, err.Error())
		} else {
			info.Printf("%sfreed storage pool object\n", id)
		}
	}
}

func freePool(ctx context.Context, p *libvirt.StoragePool) {

	id := getReqIDFromContext(ctx)

	err := p.Free()
	if err != nil {
		fail.Printf("%sfailed to free storage pool object: %s\n", id, err.Error())
	} else {
		info.Printf("%sfreed storage pool object\n", id)
	}
}

func isStorageAvailable(ctx context.Context, c *libvirt.Connect, poolName string) (bool, error) {

	id := getReqIDFromContext(ctx)

	if len(poolName) == 0 {
		fail.Printf("%sstorage pool name can not be empty\n", id)
		return false, fmt.Errorf("storage pool name can not be empty")
	}

	pool, err := lookupPoolByName(ctx, c, poolName)
	if err != nil {
		return false, err
	}
	defer freePool(ctx, pool)

	poolInfo, err := getPoolInfo(ctx, pool)
	if err != nil {
		return false, err
	}

	if poolInfo.State != libvirt.STORAGE_POOL_RUNNING {
		fail.Printf("%sstorage pool %s is not running normally\n", id, poolName)
		return false, fmt.Errorf("storage pool %s is not running normally", poolName)
	}

	if poolInfo.Available < 50*1024*1024*1024 {
		fail.Printf("%sstorage pool %s free space at critical levels: %d bytes\n", id, poolName, poolInfo.Available)
		return false, fmt.Errorf("storage pool %s free space at critical: levels %d bytes", poolName, poolInfo.Available)
	}

	info.Printf("%sstorage %s free space: %d bytes\n", id, poolName, poolInfo.Available)
	return true, nil
}

func isTemplateInsideStorageAvailable(ctx context.Context, c *libvirt.Connect, storage, template string) (bool, error) {

	id := getReqIDFromContext(ctx)

	pool, err := lookupPoolByName(ctx, c, storage)
	if err != nil {
		return false, err
	}
	defer freePool(ctx, pool)

	volumes, err := listPoolVolumes(ctx, pool)
	if err != nil {
		return false, err
	}

	for _, v := range volumes {
		if v == template {
			info.Printf("%sstorage: %s has template: %s\n", id, storage, template)
			return true, nil
		}
	}

	fail.Printf("%sfailed to find template: %s in storage %s\n", id, template, storage)
	return false, fmt.Errorf("failed to find template: %s in storage %s", template, storage)
}

func lookupStorageVolByName(ctx context.Context, c *libvirt.Connect, p *libvirt.StoragePool, name string) (*libvirt.StorageVol, error) {

	id := getReqIDFromContext(ctx)

	vol, err := p.LookupStorageVolByName(name)
	if err != nil {
		fail.Printf("%sfailed to get storage volume object for %s: %s\n", id, name, err.Error())
		return nil, err
	}

	info.Printf("%sacquired storage volume object for: %s\n", id, name)
	return vol, nil
}
