package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/libvirt/libvirt-go"
	"github.com/libvirt/libvirt-go-xml"
)

/* global variable declaration, if any... */
const domainAffectConfig = "DOMAIN_AFFECT_CONFIG"
const domainAffectLive = "DOMAIN_AFFECT_LIVE"
const domainAffectCurrent = "DOMAIN_AFFECT_CURRENT"

// https://libvirt.org/formatdomain.html

func getNumOfDomains(ctx context.Context, c *libvirt.Connect) (uint32, error) {

	id := getReqIDFromContext(ctx)

	count, err := c.NumOfDomains()
	if err != nil {
		fail.Printf("%sfailed to get total number of active domains on hypervisor node: %s\n", id, err.Error())
		return 0, err
	}

	info.Printf("%sacquired total number of active domains on hypervisor node: %d\n", id, count)
	return uint32(count), nil
}

func destroyAndUndefineDomain(ctx context.Context, c *libvirt.Connect, d *libvirt.Domain, flags libvirt.DomainDestroyFlags) (bool, error) {

	id := getReqIDFromContext(ctx)

	isPersistent := isDomainPersistent(ctx, d)
	isActive := isDomainActive(ctx, d)

	if isActive {
		err := d.DestroyFlags(flags)

		if err != nil {
			fail.Printf("%sfailed to destroy domain: %s\n", id, err.Error())
			return false, err
		}

		info.Printf("%sdomain destroyed\n", id)
	}

	if isPersistent {
		err := d.UndefineFlags(libvirt.DOMAIN_UNDEFINE_MANAGED_SAVE |
			libvirt.DOMAIN_UNDEFINE_SNAPSHOTS_METADATA |
			libvirt.DOMAIN_UNDEFINE_NVRAM)

		if err != nil {
			fail.Printf("%sfailed to undefine domain: %s\n", id, err.Error())
			return false, err
		}

		info.Printf("%sdomain undefined\n", id)
	}

	return true, nil
}

func listAllDomainsWithFlags(ctx context.Context, c *libvirt.Connect, flags libvirt.ConnectListAllDomainsFlags) ([]libvirt.Domain, error) {

	id := getReqIDFromContext(ctx)

	var err error

	domains, err := c.ListAllDomains(flags)
	if err != nil {
		fail.Printf("%sfailed to get list of domains: %s\n", id, err.Error())
		return []libvirt.Domain{}, err
	}

	if len(domains) == 0 {
		fail.Printf("%sfailed to get list of domains: no domains found\n", id)
		return []libvirt.Domain{}, nil
	}

	info.Printf("%sacquired list of domains\n", id)
	return domains, nil
}

func getDomainUUID(ctx context.Context, d *libvirt.Domain) string {

	id := getReqIDFromContext(ctx)

	uuid, err := d.GetUUIDString()
	if err != nil {
		fail.Printf("%sfailed to get domain UUID: %s\n", id, err.Error())
		return ""
	}

	info.Printf("%sacquired domain UUID %s\n", id, uuid)
	return uuid
}

func getDomainName(ctx context.Context, d *libvirt.Domain) string {

	id := getReqIDFromContext(ctx)

	name, err := d.GetName()
	if err != nil {
		fail.Printf("%sfailed to get domain name: %s\n", id, err.Error())
		return ""
	}

	info.Printf("%sacquired domain name %s\n", id, name)
	return name
}

func setDomainAutostart(ctx context.Context, d *libvirt.Domain, autoStart bool) error {

	id := getReqIDFromContext(ctx)

	err := d.SetAutostart(autoStart)
	if err != nil {
		fail.Printf("%sfailed to set autostart bool type for domain: %s\n", id, err.Error())
		return err
	}

	info.Printf("%sautostart for domain is set to %t\n", id, autoStart)
	return nil
}

func lookupDomainByName(ctx context.Context, c *libvirt.Connect, domain string) (*libvirt.Domain, error) {

	id := getReqIDFromContext(ctx)

	d, err := c.LookupDomainByName(domain)
	if err != nil {
		fail.Printf("%sfailed to find domain: %s, %s\n", id, domain, err.Error())
		return nil, err
	}

	info.Printf("%sdomain %s exists\n", id, domain)
	return d, nil
}

func destroyDomain(ctx context.Context, d *libvirt.Domain, flags libvirt.DomainDestroyFlags) error {

	id := getReqIDFromContext(ctx)

	c, err := getConnectFromDomain(ctx, d)
	defer closeConnection(ctx, c)
	if err != nil || c == nil {
		return err
	}

	paths, err := getDomainBlockDeviceNamesOrPaths(ctx, d, true)
	if err != nil {
		return err
	}

	for _, path := range paths {
		err := createBackup(ctx, c, path)
		if err != nil {
			return err
		}
	}

	_, err = destroyAndUndefineDomain(ctx, c, d, flags)
	if err != nil {
		return err
	}

	pools, err := listStorgePools(ctx, c, libvirt.CONNECT_LIST_STORAGE_POOLS_ACTIVE|libvirt.CONNECT_LIST_STORAGE_POOLS_PERSISTENT)
	defer freePools(ctx, pools)
	if err != nil {
		return err
	}

	if len(pools) == 0 {
		fail.Printf("%sfailed to get storage pool(s): %s\n", id, errors.New("pool array can not be zero sized"))
		return errors.New("pool array can not be zero sized")
	}

	for _, p := range pools {

		err := refreshPool(ctx, &p)
		if err != nil {
			continue
		}

		volumes, err := listAllStorgeVolumesInPool(ctx, &p)
		defer freeVolumes(ctx, volumes)
		if err != nil {
			continue
		}

		for _, v := range volumes {
			volPath, err := getVolumePath(ctx, &v)
			if err != nil {
				continue
			}

			for _, p := range paths {
				if p == volPath {
					err = deletePoolVolume(ctx, &v, libvirt.STORAGE_VOL_DELETE_NORMAL)
					if err != nil {
						continue
					}
				}
			}
		}
	}

	for _, path := range paths {
		v, err := lookupStorageVolByPath(ctx, c, path)
		if err == nil {
			fail.Printf("%sfailed to remove volume %s: %s\n", id, path, err.Error())
			freeVolume(ctx, v)
		}
	}

	info.Printf("%sdestroyed domain\n", id)
	return nil
}

func startDomain(ctx context.Context, d *libvirt.Domain) error {

	id := getReqIDFromContext(ctx)

	err := d.Create()
	if err != nil {
		fail.Printf("%sfailed to start domain: %s\n", id, err.Error())
		return err
	}

	info.Printf("%sstarted domain\n", id)
	return nil
}

func resetDomain(ctx context.Context, d *libvirt.Domain) error {

	id := getReqIDFromContext(ctx)

	err := d.Reset(0)
	if err != nil {
		fail.Printf("%sfailed to hard-reset domain: %s\n", id, err.Error())
		return err
	}

	info.Printf("%shard-reseted domain\n", id)
	return nil
}

func shutdownDomain(ctx context.Context, d *libvirt.Domain, flag libvirt.DomainShutdownFlags) error {

	id := getReqIDFromContext(ctx)

	err := d.ShutdownFlags(flag)
	if err != nil {
		fail.Printf("%sfailed to shutdown domain: %s\n", id, err.Error())
		return err
	}

	info.Printf("%sdomain was shutdown\n", id)
	return nil
}

func rebootDomain(ctx context.Context, d *libvirt.Domain, flag libvirt.DomainRebootFlagValues) error {

	id := getReqIDFromContext(ctx)

	err := d.Reboot(flag)
	if err != nil {
		fail.Printf("%sfailed to reboot domain: %s\n", id, err.Error())
		return err
	}

	info.Printf("%srebooted domain\n", id)
	return nil
}

func freeDomains(ctx context.Context, d []libvirt.Domain) {

	id := getReqIDFromContext(ctx)

	for _, e := range d {
		err := e.Free()
		if err != nil {
			fail.Printf("%sfailed to free domain object: %s\n", id, err.Error())
		} else {
			info.Printf("%sfreed domain object\n", id)
		}
	}
}

func freeDomain(ctx context.Context, d *libvirt.Domain) {

	id := getReqIDFromContext(ctx)

	err := d.Free()
	if err != nil {
		fail.Printf("%sfailed to free domain object: %s\n", id, err.Error())
	}

	info.Printf("%sfreed domain object\n", id)
}

func getDomainsStats(ctx context.Context, c *libvirt.Connect, d []*libvirt.Domain, flags libvirt.DomainStatsTypes) ([]libvirt.DomainStats, error) {

	id := getReqIDFromContext(ctx)

	s, err := c.GetAllDomainStats(d, flags, 0)
	if err != nil || len(s) == 0 {
		fail.Printf("%sfailed to get stats for domain(s): %s\n", id, err.Error())
		return []libvirt.DomainStats{}, err
	}

	info.Printf("%sacquired stats for domain(s)\n", id)
	return s, nil
}

func isDomainExists(ctx context.Context, c *libvirt.Connect, domain string) bool {

	d, err := lookupDomainByName(ctx, c, domain)
	if err != nil {
		return false
	}
	defer freeDomain(ctx, d)

	return true
}

func isDomainActive(ctx context.Context, d *libvirt.Domain) bool {

	id := getReqIDFromContext(ctx)

	s, err := d.IsActive()
	if err != nil {
		fail.Printf("%sfailed to get active status for domain: %s\n", id, err.Error())
		return false
	}

	info.Printf("%sacquired active status for domain\n", id)
	return s
}

func isDomainPersistent(ctx context.Context, d *libvirt.Domain) bool {

	id := getReqIDFromContext(ctx)

	s, err := d.IsPersistent()
	if err != nil {
		fail.Printf("%sfailed to get persistent status for domain: %s\n", id, err.Error())
		return false
	}

	info.Printf("%sacquired persistent status for domain\n", id)
	return s
}

func isDomainUpdated(ctx context.Context, d *libvirt.Domain) bool {

	id := getReqIDFromContext(ctx)

	s, err := d.IsUpdated()
	if err != nil {
		fail.Printf("%sfailed to get updated status for domain: %s\n", id, err.Error())
		return false
	}

	info.Printf("%sacquired updated status for domain\n", id)
	return s
}

func isDomainAutostarted(ctx context.Context, d *libvirt.Domain) bool {

	id := getReqIDFromContext(ctx)

	s, err := d.GetAutostart()
	if err != nil {
		fail.Printf("%sfailed to get autostarted status for domain: %s\n", id, err.Error())
		return false
	}

	info.Printf("%sacquired autostarted status for domain\n", id)
	return s
}

func getDomainSecurityStatus(ctx context.Context, d *libvirt.Domain) string {

	id := getReqIDFromContext(ctx)

	s, err := d.GetSecurityLabel()
	if err != nil {
		fail.Printf("%sfailed to get security status for domain: %s\n", id, err.Error())
		return ""
	}

	info.Printf("%sacquired security status for domain\n", id)
	return s.Label
}

func getDomainHypervisorType(ctx context.Context, d *libvirt.Domain) (string, error) {

	id := getReqIDFromContext(ctx)

	xmlDoc, err := d.GetXMLDesc(0)
	if err != nil {
		fail.Printf("%sfailed to get hypervisor type for domain: %s\n", id, err.Error())
		return unknown, err
	}
	info.Printf("%sacquired hypervisor XML\n", id)

	domCfg := &libvirtxml.Domain{}
	err = domCfg.Unmarshal(xmlDoc)
	if err != nil {
		fail.Printf("%sfailed to get hypervisor type for domain: %s\n", id, err.Error())
		return unknown, err
	}

	info.Printf("%sacquired hypervisor type %s for domain\n", id, domCfg.Type)
	return strings.ToUpper(domCfg.Type), nil
}

func getNewDomainImageName(ctx context.Context, c *libvirt.Connect, domainName, storagePoolName string) (string, error) {

	pool, err := lookupPoolByName(ctx, c, storagePoolName)
	if err != nil {
		return "", err
	}
	defer freePool(ctx, pool)

	poolPath, err := getPoolPath(ctx, pool)
	if err != nil {
		return "", err
	}

	imagePath := filepath.Clean(fmt.Sprintf("%s/%s.qcow2", poolPath, domainName))
	vol, err := lookupStorageVolByPath(ctx, c, imagePath)
	if err == nil {
		defer freeVolume(ctx, vol)
		return "", fmt.Errorf("image: %s exists", imagePath)
	}

	return imagePath, nil
}

func isDomainNameValidAndAvailable(ctx context.Context, c *libvirt.Connect, name string) (bool, error) {

	id := getReqIDFromContext(ctx)

	if len(name) == 0 {
		fail.Printf("%sdomain name is empty\n", id)
		return false, fmt.Errorf("domain name is empty")
	}

	namePattern := "([0-9a-zA-Z]|-|_)+"
	ok, err := regexp.Match(namePattern, []byte(name))

	if err != nil {
		fail.Printf("%snot valid name, should contain only this symbols: (0-9,a-z,A-Z,_,-): %s: %s\n", id, name, err.Error())
		return false, fmt.Errorf("not valid name, should contain only this symbols: (0-9,a-z,A-Z,_,-): %s: %s", name, err.Error())
	}

	if !ok {
		fail.Printf("%snot valid name, should contain only this symbols: (0-9,a-z,A-Z,_,-): %s\n", id, name)
		return false, fmt.Errorf("not valid name, should contain only this symbols: (0-9,a-z,A-Z,_,-): %s", name)
	}

	ok = isDomainExists(ctx, c, name)
	if ok {
		return false, fmt.Errorf("domain: %s already exists", name)
	}

	info.Printf("%svalid name: %s\n", id, name)
	return true, nil
}

func prepareXMLforNewDomain(ctx context.Context, c *libvirt.Connect, uuid, name string, vCPU, maxVCPUs int, memory, maxMemory uint, storagePool, network, mac string, vlan uint) (string, error) {

	id := getReqIDFromContext(ctx)

	imagePath, err := getNewDomainImageName(ctx, c, name, storagePool)
	if err != nil {
		fail.Printf("%sfailed to allocate name for domain image: %s\n", id, err.Error())
		return "", err
	}

	info.Printf("%sallocated name for domain image: %s\n", id, imagePath)

	xml := `
    <domain type='kvm'>
    <name>TEMPLATE</name>
    <uuid>0d15ea5e-dead-dead-dead-defec8eddead</uuid>
    <metadata>
      <my:custom xmlns:my="1c5537ac-8c84-4313-a8e7-9dd8d45ac7ed">
        <my:network type="max_tx_rate">100</my:network>
        <my:network type="trust">off</my:network>
        <my:network type="spoofchk">on</my:network>
        <my:network type="query_rss">off</my:network>
        <my:network type="qos">0</my:network>
      </my:custom>
    </metadata>
    <memory unit='KiB'>10240</memory>
    <currentMemory unit='KiB'>10240</currentMemory>
    <vcpu placement='static' current='1'>1</vcpu>
    <cputune>
      <shares>1024</shares>
    </cputune>
    <sysinfo type='smbios'>
      <bios>
        <entry name='vendor'>KVM</entry>
      </bios>
      <system>
        <entry name='manufacturer'>KVM</entry>
        <entry name='product'>VM</entry>
      </system>
      <baseBoard>
        <entry name='manufacturer'>KVM</entry>
        <entry name='product'>VM</entry>
      </baseBoard>
    </sysinfo>
    <os>
      <type arch='x86_64'>hvm</type>
      <boot dev='hd'/>
      <smbios mode='sysinfo'/>
    </os>
    <features>
      <acpi/>
      <apic/>
    </features>
    <cpu mode='host-model' check='partial'>
      <model fallback='allow'/>
    </cpu>
    <clock offset='utc'>
      <timer name='rtc' tickpolicy='catchup'/>
      <timer name='pit' tickpolicy='delay'/>
      <timer name='hpet' present='no'/>
    </clock>
    <on_poweroff>destroy</on_poweroff>
    <on_reboot>restart</on_reboot>
    <on_crash>restart</on_crash>
    <pm>
      <suspend-to-mem enabled='yes'/>
      <suspend-to-disk enabled='no'/>
    </pm>
    <devices>
      <emulator>/usr/bin/kvm-spice</emulator>
      <controller type='scsi' index='0' model='virtio-scsi'/>
      <controller type='usb' index='0' model='ich9-ehci1'/>
      <controller type='usb' index='0' model='ich9-uhci1'>
        <master startport='0'/>
      </controller>
      <controller type='pci' index='0' model='pci-root'/>
      <controller type='ide' index='0'/>
      <controller type='virtio-serial' index='0'/>
      <serial type='pty'>
        <target port='0'/>
      </serial>
      <console type='pty'>
        <target type='serial' port='0'/>
      </console>
      <channel type='unix'>
        <target type='virtio' name='org.qemu.guest_agent.0'/>
      </channel>
      <input type='keyboard' bus='virtio'/>
      <input type='mouse' bus='virtio'/>
      <memballoon model='virtio'>
        <stats period='3'/>
      </memballoon>
    </devices>
	</domain>`

	domCfg := &libvirtxml.Domain{}
	err = domCfg.Unmarshal(xml)
	if err != nil {
		fail.Printf("%sfailed to unmarshal domain XML: %s\n", id, err.Error())
		return "", err
	}

	domCfg.Name = name
	domCfg.UUID = uuid

	if domCfg.Memory != nil {
		domCfg.Memory.Value = maxMemory
	}

	if domCfg.CurrentMemory != nil {
		domCfg.CurrentMemory.Value = memory
	}

	if domCfg.VCPU != nil {
		domCfg.VCPU.Current = strconv.Itoa(vCPU)
		domCfg.VCPU.Value = maxVCPUs
	}

	if domCfg.Devices != nil {
		/*
			<disk type='file' device='disk'>
				<driver name='qemu' type='qcow2' cache='directsync' error_policy='enospace' rerror_policy='stop' discard='unmap'/>
				<source file='/var/lib/libvirt/images/ubuntu-16.04.qcow2'/>
				<target dev='sda' bus='scsi'/>
				<iotune>
					<read_iops_sec>1000</read_iops_sec>
					<write_iops_sec>400</write_iops_sec>
					<read_iops_sec_max>1100</read_iops_sec_max>
					<write_iops_sec_max>450</write_iops_sec_max>
					<read_iops_sec_max_length>15</read_iops_sec_max_length>
					<write_iops_sec_max_length>5</write_iops_sec_max_length>
				</iotune>
			</disk>
		*/
		domCfg.Devices.Disks = []libvirtxml.DomainDisk{
			libvirtxml.DomainDisk{
				Device: "disk",
				Driver: &libvirtxml.DomainDiskDriver{
					Name:         "qemu",
					Type:         "qcow2",
					Cache:        "directsync",
					ErrorPolicy:  "enospace",
					RErrorPolicy: "stop",
					Discard:      "unmap",
				},
				Source: &libvirtxml.DomainDiskSource{
					File: &libvirtxml.DomainDiskSourceFile{
						File: imagePath,
					},
				},
				Target: &libvirtxml.DomainDiskTarget{
					Dev: "sda",
					Bus: "scsi",
				},
				IOTune: &libvirtxml.DomainDiskIOTune{
					ReadIopsSec:           1000,
					WriteIopsSec:          400,
					ReadIopsSecMax:        1100,
					WriteIopsSecMax:       450,
					ReadIopsSecMaxLength:  15,
					WriteIopsSecMaxLength: 5,
				},
			},
		}
	}

	if domCfg.Devices != nil {
		/*
			<interface type="network">
				<mac address="52:54:00:ff:ff:ff"></mac>
				<source network="pf-enp6s2f2"></source>
				<vlan>
					<tag id="222"></tag>
				</vlan>
			</interface>
		*/
		domCfg.Devices.Interfaces = []libvirtxml.DomainInterface{
			libvirtxml.DomainInterface{
				MAC: &libvirtxml.DomainInterfaceMAC{
					Address: mac,
				},
				Source: &libvirtxml.DomainInterfaceSource{
					Network: &libvirtxml.DomainInterfaceSourceNetwork{
						Network: network,
					},
				},
				VLan: &libvirtxml.DomainInterfaceVLan{
					Tags: []libvirtxml.DomainInterfaceVLanTag{
						libvirtxml.DomainInterfaceVLanTag{
							ID: vlan,
						},
					},
				},
			},
		}
	}

	xml, err = domCfg.Marshal()
	if err != nil {
		fail.Printf("%sfailed to marshal domain XML: %s\n", id, err.Error())
		return "", err
	}

	return xml, nil
}
