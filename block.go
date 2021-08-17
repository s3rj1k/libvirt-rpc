package main

import (
	"context"

	"github.com/libvirt/libvirt-go"
)

/* global variable declaration, if any... */
const domainBlockJobTypeActiveCommit = "DOMAIN_BLOCK_JOB_TYPE_ACTIVE_COMMIT"
const domainBlockJobTypeCommit = "DOMAIN_BLOCK_JOB_TYPE_COMMIT"
const domainBlockJobTypeCopy = "DOMAIN_BLOCK_JOB_TYPE_COPY"
const domainBlockJobTypePull = "DOMAIN_BLOCK_JOB_TYPE_PULL"

func getDomainBlockJobInfo(ctx context.Context, d *libvirt.Domain, disk string) (blockJobInfo, error) {

	var jobInfo blockJobInfo

	id := getReqIDFromContext(ctx)

	s, err := d.GetBlockJobInfo(disk, libvirt.DomainBlockJobInfoFlags(0))
	if err != nil || s == nil {
		fail.Printf("%sfailed to get block job info for %s: %s\n", id, disk, err.Error())
		return blockJobInfo{}, err
	}

	jobInfo.Bandwidth = s.Bandwidth
	jobInfo.Cur = s.Cur
	jobInfo.End = s.End

	switch s.Type {
	case libvirt.DOMAIN_BLOCK_JOB_TYPE_UNKNOWN:
		jobInfo.Type = ""
	case libvirt.DOMAIN_BLOCK_JOB_TYPE_PULL:
		jobInfo.Type = domainBlockJobTypePull
	case libvirt.DOMAIN_BLOCK_JOB_TYPE_COPY:
		jobInfo.Type = domainBlockJobTypeCopy
	case libvirt.DOMAIN_BLOCK_JOB_TYPE_COMMIT:
		jobInfo.Type = domainBlockJobTypeCommit
	case libvirt.DOMAIN_BLOCK_JOB_TYPE_ACTIVE_COMMIT:
		jobInfo.Type = domainBlockJobTypeActiveCommit
	default:
		jobInfo.Type = ""
	}

	return jobInfo, nil
}

func setDomainBlockIoTune(ctx context.Context, d *libvirt.Domain, dev string, read uint64, write uint64) error {

	id := getReqIDFromContext(ctx)

	flags := libvirt.DOMAIN_AFFECT_CURRENT

	if ok := isDomainPersistent(ctx, d); ok {
		flags = flags | libvirt.DOMAIN_AFFECT_CONFIG
	}

	if ok := isDomainActive(ctx, d); ok {
		flags = flags | libvirt.DOMAIN_AFFECT_LIVE
	}

	var s libvirt.DomainBlockIoTuneParameters

	s.ReadIopsSecSet = true
	s.ReadIopsSec = read
	s.ReadIopsSecMaxSet = true
	s.ReadIopsSecMax = read + 100
	s.ReadIopsSecMaxLengthSet = true
	s.ReadIopsSecMaxLength = 15
	s.WriteIopsSecSet = true
	s.WriteIopsSec = write
	s.WriteIopsSecMaxSet = true
	s.WriteIopsSecMax = write + 50
	s.WriteIopsSecMaxLengthSet = true
	s.WriteIopsSecMaxLength = 5

	err := d.SetBlockIoTune(dev, &s, flags)
	if err != nil {
		fail.Printf("%sfailed to set block device %s IO policy for domain: %s\n", id, dev, err.Error())
		return err
	}
	info.Printf("%sblock device %s Read IOPS set to %d\n", id, dev, read)
	info.Printf("%sblock device %s Write IOPS set to %d\n", id, dev, write)

	return nil
}

/*
virsh help blkdeviotune
TEST: fio --randrepeat=1 --ioengine=libaio --direct=1 --gtod_reduce=1 --name=test --filename=test --bs=4k --iodepth=64 --size=8G --readwrite=randwrite --runtime=60
*/
func getDomainBlockIoTune(ctx context.Context, d *libvirt.Domain, dev string, flags libvirt.DomainModificationImpact) blockIO {

	id := getReqIDFromContext(ctx)

	var ret blockIO

	s, err := d.GetBlockIoTune(dev, flags)
	if err != nil {
		fail.Printf("%sfailed to get block IO tune values for: %s, with flag: %d, %s\n", id, dev, flags, err.Error())
		return blockIO{}
	}

	switch flags {
	case libvirt.DOMAIN_AFFECT_CURRENT:
		ret.ModificationImpact = domainAffectCurrent
	case libvirt.DOMAIN_AFFECT_CONFIG:
		ret.ModificationImpact = domainAffectConfig
	case libvirt.DOMAIN_AFFECT_LIVE:
		ret.ModificationImpact = domainAffectLive
	default:
		ret.ModificationImpact = domainAffectCurrent
	}

	ret.ReadBytesSec = s.ReadBytesSec
	ret.ReadBytesSecMax = s.ReadBytesSecMax
	ret.ReadBytesSecMaxLength = s.ReadBytesSecMaxLength
	ret.ReadIopsSec = s.ReadIopsSec
	ret.ReadIopsSecMax = s.ReadIopsSecMax
	ret.ReadIopsSecMaxLength = s.ReadIopsSecMaxLength
	ret.SizeIopsSec = s.SizeIopsSec
	ret.TotalBytesSec = s.TotalBytesSec
	ret.TotalBytesSecMax = s.TotalBytesSecMax
	ret.TotalBytesSecMaxLength = s.TotalBytesSecMaxLength
	ret.TotalIopsSec = s.TotalIopsSec
	ret.TotalIopsSecMax = s.TotalIopsSecMax
	ret.TotalIopsSecMaxLength = s.TotalIopsSecMaxLength
	ret.WriteBytesSec = s.WriteBytesSec
	ret.WriteBytesSecMax = s.WriteBytesSecMax
	ret.WriteBytesSecMaxLength = s.WriteBytesSecMaxLength
	ret.WriteIopsSec = s.WriteIopsSec
	ret.WriteIopsSecMax = s.WriteIopsSecMax
	ret.WriteIopsSecMaxLength = s.WriteIopsSecMaxLength
	ret.GroupName = s.GroupName

	info.Printf("%sacquired block IO tune values for: %s, with flag: %d\n", id, dev, flags)
	return ret
}

// virsh help blkiotune
func getDomainBlkioParams(ctx context.Context, d *libvirt.Domain, flags libvirt.DomainModificationImpact) blockParams {

	id := getReqIDFromContext(ctx)

	var ret blockParams

	s, err := d.GetBlkioParameters(flags)
	if err != nil {
		fail.Printf("%sfailed to get block parameters for domain with flag: %d, %s\n", id, flags, err.Error())
		return blockParams{}
	}

	switch flags {
	case libvirt.DOMAIN_AFFECT_CURRENT:
		ret.ModificationImpact = domainAffectCurrent
	case libvirt.DOMAIN_AFFECT_CONFIG:
		ret.ModificationImpact = domainAffectConfig
	case libvirt.DOMAIN_AFFECT_LIVE:
		ret.ModificationImpact = domainAffectLive
	default:
		ret.ModificationImpact = domainAffectCurrent
	}

	ret.Weight = s.Weight
	ret.DeviceWeight = s.DeviceWeight
	ret.DeviceReadIops = s.DeviceReadIops
	ret.DeviceWriteIops = s.DeviceWriteIops
	ret.DeviceReadBps = s.DeviceReadBps
	ret.DeviceWriteBps = s.DeviceWriteBps

	info.Printf("%sacquired block parameters for domain with flag: %d\n", id, flags)
	return ret
}
