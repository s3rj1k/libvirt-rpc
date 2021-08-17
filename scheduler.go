package main

import (
	"context"

	"github.com/libvirt/libvirt-go"
)

func setDomainSchedulerCPUShares(ctx context.Context, d *libvirt.Domain, CPUShares uint64) error {

	id := getReqIDFromContext(ctx)

	flags := libvirt.DOMAIN_AFFECT_CURRENT

	if ok := isDomainPersistent(ctx, d); ok {
		flags = flags | libvirt.DOMAIN_AFFECT_CONFIG
	}

	if ok := isDomainActive(ctx, d); ok {
		flags = flags | libvirt.DOMAIN_AFFECT_LIVE
	}

	var s libvirt.DomainSchedulerParameters
	s.CpuShares = CPUShares
	s.CpuSharesSet = true

	err := d.SetSchedulerParametersFlags(&s, flags)
	if err != nil {
		fail.Printf("%sfailed to set scheduler CPU shares for domain: %s\n", id, err.Error())
		return err
	}

	info.Printf("%sscheduler CPU shares value for domain set to %d\n", id, CPUShares)
	return nil
}

// virsh help schedinfo
func getDomainSchedulerInfo(ctx context.Context, d *libvirt.Domain, flags libvirt.DomainModificationImpact) (schedulerInfo, error) {

	id := getReqIDFromContext(ctx)

	var ret schedulerInfo

	s, err := d.GetSchedulerParametersFlags(flags)
	if err != nil {
		fail.Printf("%sfailed to get scheduler info parameters for domain with flag: %d, %s\n", id, flags, err.Error())
		return schedulerInfo{}, err
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

	ret.Type = s.Type
	ret.CPUShares = s.CpuShares
	ret.GlobalPeriod = s.GlobalPeriod
	ret.GlobalQuota = s.GlobalQuota
	ret.VcpuPeriod = s.VcpuPeriod
	ret.VcpuQuota = s.VcpuQuota
	ret.EmulatorPeriod = s.EmulatorPeriod
	ret.EmulatorQuota = s.EmulatorQuota
	ret.IothreadPeriod = s.IothreadPeriod
	ret.IothreadQuota = s.IothreadQuota
	ret.Weight = s.Weight
	ret.Cap = s.Cap
	ret.Reservation = s.Reservation
	ret.Limit = s.Limit
	ret.Shares = s.Shares

	info.Printf("%sacquired scheduler info parameters for domain with flag: %d\n", id, flags)
	return ret, nil
}
