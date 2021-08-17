package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/libvirt/libvirt-go"
	"github.com/semrush/zenrpc"
)

func getReqIDFromContext(ctx context.Context) string {
	if id := zenrpc.IDFromContext(ctx); id != nil {
		return fmt.Sprintf("id=%s, ", string(*id))
	}
	return ""
}

//go:generate zenrpc

// Ping - answers to jrpc ping method
func (as JRPCService) Ping() bool {
	return true
}

// GenUUID - generates pseudo-random UUID
func (as JRPCService) GenUUID(ctx context.Context) string {
	return genUUID(ctx)
}

// GenMAC - generates random QEMU-KVM MAC
func (as JRPCService) GenMAC(ctx context.Context) string {
	return genMAC(ctx)
}

// Lock - add lock for domain (hash)
func (as JRPCService) Lock(ctx context.Context, Domain string) bool {
	return addLock(ctx, Domain)
}

// UnLock - remove lock from domain (hash)
func (as JRPCService) UnLock(ctx context.Context, Domain string) bool {
	return removeLock(ctx, Domain)
}

// ListLocks - list current locks
func (as JRPCService) ListLocks(ctx context.Context) []string {
	return listLocks(ctx)
}

// HypervisorInfo - acquires info from hypervisor
func (as JRPCService) HypervisorInfo(ctx context.Context) (NodeInfoResponse, error) {

	isLocked := isLockedAndMakeLock(ctx, "Local Hypervisor", 10)
	if isLocked {
		return NodeInfoResponse{}, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "ro")
	if err != nil {
		return NodeInfoResponse{}, err
	}
	defer closeConnection(ctx, c)

	r, err := getNodeInfoResponse(ctx, c)
	if err != nil {
		return NodeInfoResponse{}, err
	}

	return r, nil
}

// RefreshAllStorgePools - refreshes usage statistics for all directory based storage pools
func (as JRPCService) RefreshAllStorgePools(ctx context.Context) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, "Local Hypervisor", 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	err = refreshAllStorgePools(ctx, c)
	if err != nil {
		return false, err
	}

	return true, nil
}

// Info - acquires metric(s) and info from single domain
func (as JRPCService) Info(ctx context.Context, Domain string) (InfoResponse, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return InfoResponse{}, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "ro")
	if err != nil {
		return InfoResponse{}, err
	}
	defer closeConnection(ctx, c)

	dom, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return InfoResponse{}, err
	}
	defer freeDomain(ctx, dom)

	flags := libvirt.DOMAIN_STATS_BALLOON |
		libvirt.DOMAIN_STATS_BLOCK |
		libvirt.DOMAIN_STATS_CPU_TOTAL |
		libvirt.DOMAIN_STATS_STATE |
		libvirt.DOMAIN_STATS_VCPU

	doms := make([]*libvirt.Domain, 0, 1)
	doms = append(doms, dom)

	s, err := getDomainsStats(ctx, c, doms, flags)
	if err != nil {
		return InfoResponse{}, err
	}

	r := getDomainsInfoResponse(ctx, s, len(s))
	if len(r) != 1 {
		return InfoResponse{}, errors.New("domain stats array length must equal to 1")
	}

	return r[0], nil
}

// QemuAgentInfo - refreshes usage statistics for all directory based storage pools
func (as JRPCService) QemuAgentInfo(ctx context.Context, Domain string) (QemuAgentResponse, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return QemuAgentResponse{}, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return QemuAgentResponse{}, err
	}
	defer closeConnection(ctx, c)

	d, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return QemuAgentResponse{}, err
	}
	defer freeDomain(ctx, d)

	isActive := isDomainActive(ctx, d)
	if !isActive {
		return QemuAgentResponse{}, errors.New("domain must be active while setting maximum memory value")
	}

	ok, err := isDomainBlockJobRunning(ctx, d)
	if err != nil {
		return QemuAgentResponse{}, err
	}
	if ok {
		return QemuAgentResponse{}, errors.New("sanity lock, block device job is currently in process")
	}

	r := getQemuAgentInfoResponse(ctx, d)

	return r, nil
}

// Domains - acquires list of domains
func (as JRPCService) Domains(ctx context.Context, Search string) ([]string, error) {

	isLocked := isLockedAndMakeLock(ctx, "Local Hypervisor", 10)
	if isLocked {
		return []string{}, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "ro")
	if err != nil {
		return []string{}, err
	}
	defer closeConnection(ctx, c)

	domObjs, err := listAllDomainsWithFlags(ctx, c, libvirt.ConnectListAllDomainsFlags(0))
	if err != nil {
		return []string{}, err
	}
	defer freeDomains(ctx, domObjs)

	domains := make([]string, 0, len(domObjs))

	for _, d := range domObjs {

		name := getDomainName(ctx, &d)

		if len(name) == 0 {
			continue
		}

		if len(Search) == 0 {
			domains = append(domains, name)
			continue
		}

		if strings.HasPrefix(strings.ToLower(name), strings.ToLower(Search)) {
			domains = append(domains, name)
			continue
		}
	}

	return domains, nil
}

// SetPVIDForNetworkDevice - sets PVID for hostdev network device, triggers device remove-attach cycle
func (as JRPCService) SetPVIDForNetworkDevice(ctx context.Context, Domain string, MAC string, PVID uint) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	d, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return false, err
	}
	defer freeDomain(ctx, d)

	ok, err := isDomainBlockJobRunning(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, block device job is currently in process")
	}

	ok, err = setPVIDForDomainNetworkDevice(ctx, d, MAC, PVID)
	if err != nil || !ok {
		return false, err
	}

	return true, nil
}

// SetNetworkSpeed - sets spped for all hostdev network devices bound to specified domain
func (as JRPCService) SetNetworkSpeed(ctx context.Context, Domain string, Speed uint) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	d, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return false, err
	}
	defer freeDomain(ctx, d)

	ok, err := isDomainBlockJobRunning(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, block device job is currently in process")
	}

	ok, err = setDomainMetadataNetworkRate(ctx, d, Speed)
	if err != nil || !ok {
		return false, err
	}

	return true, nil
}

// SetPassword - sets user password inside domain using Guest Agent
func (as JRPCService) SetPassword(ctx context.Context, Domain string, VMUser string, VMPassword string) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	d, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return false, err
	}
	defer freeDomain(ctx, d)

	ok, err := isDomainBlockJobRunning(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, block device job is currently in process")
	}

	err = setGuestPassword(ctx, d, VMUser, VMPassword)
	if err != nil {
		return false, err
	}

	return true, nil
}

// SetMemory - sets current available memory [KiB] for domain
func (as JRPCService) SetMemory(ctx context.Context, Domain string, Memory uint64) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	d, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return false, err
	}
	defer freeDomain(ctx, d)

	ok, err := isDomainBlockJobRunning(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, block device job is currently in process")
	}

	err = setDomainCurrentMemory(ctx, d, Memory)
	if err != nil {
		return false, err
	}

	return true, nil
}

// SetMemoryStatsPeriod - sets period of memory stats collection for domain in seconds
func (as JRPCService) SetMemoryStatsPeriod(ctx context.Context, Domain string, Period int) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	d, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return false, err
	}
	defer freeDomain(ctx, d)

	ok, err := isDomainBlockJobRunning(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, block device job is currently in process")
	}

	err = setDomainMemoryStatsPeriod(ctx, d, Period)
	if err != nil {
		return false, err
	}

	return true, nil
}

// SetMaxMemory - sets maximum available memory [KiB] for domain
func (as JRPCService) SetMaxMemory(ctx context.Context, Domain string, Memory uint64) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	d, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return false, err
	}
	defer freeDomain(ctx, d)

	isActive := isDomainActive(ctx, d)
	if isActive {
		return false, errors.New("domain must not be active while setting maximum memory value")
	}

	ok, err := isDomainBlockJobRunning(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, block device job is currently in process")
	}

	err = setDomainMaxMemory(ctx, d, Memory)
	if err != nil {
		return false, err
	}

	return true, nil
}

// SetVCPUs - sets current available vCPUs for domain
func (as JRPCService) SetVCPUs(ctx context.Context, Domain string, VCPUsNum uint) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	d, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return false, err
	}
	defer freeDomain(ctx, d)

	ok, err := isDomainBlockJobRunning(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, block device job is currently in process")
	}

	err = setDomainCurrentVCPUs(ctx, d, VCPUsNum)
	if err != nil {
		return false, err
	}

	return true, nil
}

// SetMaxVCPUs - sets maximum available vCPUs for domain
func (as JRPCService) SetMaxVCPUs(ctx context.Context, Domain string, VCPUsNum uint) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	d, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return false, err
	}
	defer freeDomain(ctx, d)

	isActive := isDomainActive(ctx, d)
	if isActive {
		return false, errors.New("domain must not be active while setting maximum vCPU value")
	}

	ok, err := isDomainBlockJobRunning(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, block device job is currently in process")
	}

	err = setDomainMaxVCPUs(ctx, d, VCPUsNum)
	if err != nil {
		return false, err
	}

	return true, nil
}

// SetDomainSchedulerCPUShares - sets scheduler CPU shares for domain
func (as JRPCService) SetDomainSchedulerCPUShares(ctx context.Context, Domain string, CPUShares uint64) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	d, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return false, err
	}
	defer freeDomain(ctx, d)

	ok, err := isDomainBlockJobRunning(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, block device job is currently in process")
	}

	err = setDomainSchedulerCPUShares(ctx, d, CPUShares)
	if err != nil {
		return false, err
	}

	return true, nil
}

// SetDomainDeviceIOPS - sets block device RW IOPS for domain, virsh help blkdeviotune
func (as JRPCService) SetDomainDeviceIOPS(ctx context.Context, Domain string, Device string, Read uint64, Write uint64) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	d, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return false, err
	}
	defer freeDomain(ctx, d)

	ok, err := isDomainBlockJobRunning(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, block device job is currently in process")
	}

	err = setDomainBlockIoTune(ctx, d, Device, Read, Write)
	if err != nil {
		return false, err
	}

	return true, nil
}

// SetAutostart - sets autostart action for domain
func (as JRPCService) SetAutostart(ctx context.Context, Domain string, Autostart bool) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	d, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return false, err
	}
	defer freeDomain(ctx, d)

	ok, err := isDomainBlockJobRunning(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, block device job is currently in process")
	}

	err = setDomainAutostart(ctx, d, Autostart)
	if err != nil {
		return false, err
	}

	return true, nil
}

// Reboot - restarts domain
func (as JRPCService) Reboot(ctx context.Context, Domain string) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	d, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return false, err
	}
	defer freeDomain(ctx, d)

	ok, err := isDomainBlockJobRunning(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, block device job is currently in process")
	}

	err = rebootDomain(ctx, d, libvirt.DOMAIN_REBOOT_DEFAULT)
	if err != nil {
		return false, err
	}

	return true, nil
}

// Shutdown - shutdowns domain
func (as JRPCService) Shutdown(ctx context.Context, Domain string) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	d, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return false, err
	}
	defer freeDomain(ctx, d)

	ok, err := isDomainBlockJobRunning(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, block device job is currently in process")
	}

	err = shutdownDomain(ctx, d, libvirt.DOMAIN_SHUTDOWN_DEFAULT)
	if err != nil {
		return false, err
	}

	return true, nil
}

// Reset - resets domain
func (as JRPCService) Reset(ctx context.Context, Domain string) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	d, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return false, err
	}
	defer freeDomain(ctx, d)

	ok, err := isDomainBlockJobRunning(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, block device job is currently in process")
	}

	err = resetDomain(ctx, d)
	if err != nil {
		return false, err
	}

	return true, nil
}

// Start - starts domain
func (as JRPCService) Start(ctx context.Context, Domain string) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	d, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return false, err
	}
	defer freeDomain(ctx, d)

	ok, err := isDomainBlockJobRunning(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, block device job is currently in process")
	}

	err = startDomain(ctx, d)
	if err != nil {
		return false, err
	}

	return true, nil
}

// Destroy - destroys domain
func (as JRPCService) Destroy(ctx context.Context, Domain string) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	d, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return false, err
	}
	defer freeDomain(ctx, d)

	isActive := isDomainActive(ctx, d)
	if isActive {
		return false, errors.New("domain must not be active while being destroyed")
	}

	ok, err := isDomainBlockJobRunning(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, block device job is currently in process")
	}

	ok, err = isDomainBlockHasActiveExternalBackupSnashot(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, domain has unfinished internal backup")
	}

	err = destroyDomain(ctx, d, libvirt.DOMAIN_DESTROY_GRACEFUL)
	if err != nil {
		return false, err
	}

	return true, nil
}

// MakeSnapshot - makes snapshot of not active (shutdown) domain
func (as JRPCService) MakeSnapshot(ctx context.Context, Domain string, Name string) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	d, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return false, err
	}
	defer freeDomain(ctx, d)

	isActive := isDomainActive(ctx, d)
	if isActive {
		return false, errors.New("domain must not be active while creating internal snapshot")
	}

	ok, err := isDomainBlockJobRunning(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, block device job is currently in process")
	}

	ok, err = isDomainBlockHasActiveExternalBackupSnashot(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, domain has unfinished internal backup")
	}

	xml, err := prepareXMLForSnapshot(ctx, d, Name, true)
	if err != nil {
		return false, err
	}

	flags := libvirt.DOMAIN_SNAPSHOT_CREATE_ATOMIC | libvirt.DOMAIN_SNAPSHOT_CREATE_HALT

	ok, err = makeDomainSnapshot(ctx, d, flags, xml)
	if err != nil || !ok {
		return false, err
	}

	return true, nil
}

// RemoveSnapshot - deletes snapshot of not active (shutdown) domain
func (as JRPCService) RemoveSnapshot(ctx context.Context, Domain string, Name string) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	d, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return false, err
	}
	defer freeDomain(ctx, d)

	isActive := isDomainActive(ctx, d)
	if isActive {
		return false, errors.New("domain must not be active while deleting snapshot")
	}

	ok, err := isDomainBlockJobRunning(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, block device job is currently in process")
	}

	ok, err = isDomainBlockHasActiveExternalBackupSnashot(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, domain has unfinished internal backup")
	}

	s, err := lookupDomainSnapshotByName(ctx, d, Name)

	defer func() {

		id := getReqIDFromContext(ctx)

		err = freeSnapshot(ctx, s)

		if err != nil {
			fail.Printf("%sfailed in defer: %s", id, err.Error())
		}

	}()

	if err != nil || s == nil {
		return false, err
	}

	ok, err = deleteSnapshot(ctx, s, 0)
	if err != nil || !ok {
		return false, err
	}

	return true, nil
}

// RevertToSnapshot - reverts not active domain to specified snapshot
func (as JRPCService) RevertToSnapshot(ctx context.Context, Domain string, Name string) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	d, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return false, err
	}
	defer freeDomain(ctx, d)

	isActive := isDomainActive(ctx, d)
	if isActive {
		return false, errors.New("domain must not be active while reverting to snapshot")
	}

	ok, err := isDomainBlockJobRunning(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, block device job is currently in process")
	}

	ok, err = isDomainBlockHasActiveExternalBackupSnashot(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, domain has unfinished internal backup")
	}

	s, err := lookupDomainSnapshotByName(ctx, d, Name)
	if err != nil || s == nil {
		return false, err
	}

	ok, err = revertToSnapshot(ctx, s)
	if err != nil || !ok {
		return false, err
	}

	return true, nil
}

/*
HowTo:
  http://wiki.libvirt.org/page/Live-disk-backup-with-active-blockcommit

get active disk list:
  virsh domblklist --domain ubuntu-16.04

create external snapshot:
  virsh snapshot-create-as --domain ubuntu-16.04 --name external.snapshot --disk-only --atomic --quiesce --no-metadata

get active disk list after snapshot, external snapshot should be active:
  virsh domblklist --domain ubuntu-16.04

manually backup original active iimage file:
  cp /var/lib/libvirt/images/ubuntu-16.04.qcow2 /var/lib/libvirt/images/ubuntu-16.04.backup.qcow2

check image info:
  /usr/bin/qemu-img info /var/lib/libvirt/images/ubuntu-16.04.backup.qcow2

merge changes back:
  virsh blockcommit --domain ubuntu-16.04 --path /var/lib/libvirt/images/ubuntu-16.04.external.snapshot --active --verbose --pivot

get active disk list after merge, original disk should be active:
  virsh domblklist --domain ubuntu-16.04

view block job info for debug:
  virsh blockjob --domain ubuntu-16.04 --info --path /var/lib/libvirt/images/ubuntu-16.04.external.snapshot.qcow2

manually pivot disk after successful 1st stage of block commit, need only for API debug
  virsh blockjob --domain ubuntu-16.04 --pivot --path sda
*/

// MakeBackup - makes backup using external snapshot and blockcommit for active domain
func (as JRPCService) MakeBackup(ctx context.Context, Domain string) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, Domain, 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	id := getReqIDFromContext(ctx)

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	d, err := lookupDomainByName(ctx, c, Domain)
	if err != nil {
		return false, err
	}
	defer freeDomain(ctx, d)

	isActive := isDomainActive(ctx, d)
	if !isActive {
		return false, errors.New("domain must be active while creating backup")
	}

	ok, err := isDomainBlockJobRunning(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, block device job is currently in process")
	}

	ok, err = isDomainBlockHasActiveExternalBackupSnashot(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("sanity lock, domain has unfinished backup")
	}

	xml, err := prepareXMLForSnapshot(ctx, d, "external.snapshot.qcow2", false)
	if err != nil {
		return false, err
	}

	paths, err := getDomainBlockDeviceNamesOrPaths(ctx, d, true)
	if err != nil {
		return false, err
	}

	flags := libvirt.DOMAIN_SNAPSHOT_CREATE_DISK_ONLY |
		libvirt.DOMAIN_SNAPSHOT_CREATE_QUIESCE |
		libvirt.DOMAIN_SNAPSHOT_CREATE_ATOMIC |
		libvirt.DOMAIN_SNAPSHOT_CREATE_NO_METADATA

	ok, err = makeDomainSnapshot(ctx, d, flags, xml)
	if err != nil || !ok {
		return false, err
	}

	for _, path := range paths {
		err := createBackup(ctx, c, path)
		if err != nil {
			return false, err
		}
	}

	disks, err := getDomainBlockDeviceNamesOrPaths(ctx, d, false)
	if err != nil {
		return false, err
	}

	paths, err = getDomainBlockDeviceNamesOrPaths(ctx, d, true)
	if err != nil {
		return false, err
	}

	for _, disk := range disks {

		ok, err := blockCommitActive(ctx, d, disk)
		if err != nil || !ok {
			return false, err
		}

		ok = waitBlockCommitActive(ctx, d, disk)
		if !ok {
			continue
		}

		ok, err = blockCommitActivePivot(ctx, d, disk)
		if err != nil || !ok {
			return false, err
		}
	}

	err = deleteTemporaryExternalSnapshot(ctx, c, paths)
	if err != nil {
		return false, err
	}

	ok, err = isDomainBlockHasActiveExternalBackupSnashot(ctx, d)
	if err != nil {
		return false, err
	}
	if ok {
		return false, errors.New("T_T domain backup job failed miserably")
	}

	info.Printf("%s^_^ domain backup job magically succeeded\n", id)
	return true, nil
}

// CloneImage - clones image from (left) volume name to new (right) volume name inside storage pool specified by name
func (as JRPCService) CloneImage(ctx context.Context, Storage, LeftImageName, RightImageName string) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, fmt.Sprintf("%s|%s", Storage, LeftImageName), 60)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	err = refreshAllStorgePools(ctx, c)
	if err != nil {
		return false, err
	}

	err = cloneVolumeByPath(ctx, c, Storage, LeftImageName, RightImageName)
	if err != nil {
		return false, err
	}

	return true, nil
}

// Create - creates new domain with supplied configuration
func (as JRPCService) Create(ctx context.Context, UUID, Name string, VCPU int, Memory uint, Storage, Template, Network, MAC string, VLAN uint) (bool, error) {

	maxMemory := 2 * Memory
	maxVcpus := 16

	id := getReqIDFromContext(ctx)

	isLocked := isLockedAndMakeLock(ctx, "Local Hypervisor", 60)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "rw")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	if len(UUID) == 0 {
		UUID = genUUID(ctx)
	}

	ok, err := validateCreateDomain(ctx, c, UUID, Name, VCPU, Memory, Storage, Template, Network, MAC) // no VLAN validation
	if err != nil {
		fail.Printf("%sfailed to validate domain options: %s\n", id, err.Error())
		return false, fmt.Errorf("failed to validate domain options: %s", err.Error())
	}

	if !ok {
		fail.Printf("%sfailed to validate domain options\n", id)
		return false, fmt.Errorf("failed to validate domain options")
	}

	xml, err := prepareXMLforNewDomain(ctx, c, UUID, Name, VCPU, maxVcpus, Memory, maxMemory, Storage, Network, MAC, VLAN)
	if err != nil {
		return false, err
	}

	err = cloneVolumeByPath(ctx, c, Storage, Template, fmt.Sprintf("%s.qcow2", Name))
	if err != nil {
		return false, err
	}

	dom, err := c.DomainDefineXMLFlags(xml, libvirt.DOMAIN_DEFINE_VALIDATE)
	if err != nil {
		fail.Printf("%sfailed to define domain: %s using XML: %s\n", id, Name, err.Error())
		return false, err
	}
	defer freeDomain(ctx, dom)

	info.Printf("%sdefined domain: %s\n", id, Name)

	return true, nil
}

// CheckResources - checks if requested resources available on hypervisor
func (as JRPCService) CheckResources(ctx context.Context, Name string, VCPU int, Memory uint, Storage, Network string) (bool, error) {

	isLocked := isLockedAndMakeLock(ctx, "Local Hypervisor", 10)
	if isLocked {
		return false, errors.New("thread safety lock, function is temporarily unavailable")
	}

	c, err := openConnection(ctx, "ro")
	if err != nil {
		return false, err
	}
	defer closeConnection(ctx, c)

	ok, err := checkResources(ctx, c, Name, VCPU, Memory, Storage, Network)
	if err != nil || !ok {
		return false, err
	}

	return true, nil
}
