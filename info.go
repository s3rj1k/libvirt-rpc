package main

import (
	"context"
	"strings"
	"time"

	"github.com/libvirt/libvirt-go"
)

func getNodeInfoResponse(ctx context.Context, c *libvirt.Connect) (NodeInfoResponse, error) {

	var r NodeInfoResponse
	var err error

	r.Timestamp = time.Now().Unix()
	r.Hostname, err = getNodeHostname(ctx, c)
	if err != nil {
		r.Hostname = unknown
	}

	r.Uptime, err = getNodeUptime(ctx, c)
	if err != nil {
		r.Uptime = 0
	}

	r.LibvirtVersion, err = getNodeLibVersion(ctx, c)
	if err != nil {
		return NodeInfoResponse{}, err
	}

	nodeInfo, err := getNodeInfo(ctx, c)
	if err != nil {
		return NodeInfoResponse{}, err
	}

	r.HardwareInfo.Model = nodeInfo.Model
	r.HardwareInfo.Memory = nodeInfo.Memory
	r.HardwareInfo.Cpus = nodeInfo.Cpus
	r.HardwareInfo.MHz = nodeInfo.MHz
	r.HardwareInfo.Nodes = nodeInfo.Nodes
	r.HardwareInfo.Sockets = nodeInfo.Sockets
	r.HardwareInfo.Cores = nodeInfo.Cores
	r.HardwareInfo.Threads = nodeInfo.Threads

	cpuStats, err := getNodeCPUStats(ctx, c)
	if err != nil {
		return NodeInfoResponse{}, err
	}

	r.CPUStats.Kernel = cpuStats.Kernel
	r.CPUStats.User = cpuStats.User
	r.CPUStats.Idle = cpuStats.Idle
	r.CPUStats.Iowait = cpuStats.Iowait
	r.CPUStats.Interrupt = cpuStats.Intr
	r.CPUStats.Utilization = cpuStats.Utilization

	r.MemoryStats, err = getNodeMemoryStats(ctx)
	if err != nil {
		return NodeInfoResponse{}, err
	}

	r.ActiveDomainCount, err = getNumOfDomains(ctx, c)
	if err != nil {
		return NodeInfoResponse{}, err
	}

	r.VCPUsCount, err = getNumOfAssignedNodeVCPUs(ctx, c)
	if err != nil {
		r.VCPUsCount = 0
	}

	r.ActiveNetworkCount, err = getNumOfNetworks(ctx, c)
	if err != nil {
		return NodeInfoResponse{}, err
	}

	networks, err := listNetworks(ctx, c)
	defer freeNetworks(ctx, networks)
	if err != nil {
		return NodeInfoResponse{}, err
	}

	for _, net := range networks {

		var network nodeNetwork

		network.Name, err = getNetworkName(ctx, &net)
		if err != nil {
			continue
		}

		network.UsedVFs, network.TotalVFs, err = getNetworkVFCount(ctx, c, &net)
		if err != nil {
			continue
		}

		if network.TotalVFs < 0 {
			network.TotalVFs = 0
		}

		network.AvaliableVFs = network.TotalVFs - network.UsedVFs
		if network.AvaliableVFs < 0 {
			network.AvaliableVFs = 0
		}

		r.Network = append(r.Network, network)
	}

	// here we acquire only directory based pools
	pools, err := listStorgePools(ctx, c, libvirt.CONNECT_LIST_STORAGE_POOLS_DIR)
	defer freePools(ctx, pools)
	if err != nil {
		return NodeInfoResponse{}, err
	}

	for _, pool := range pools {

		var poolDef nodePool

		poolDef.Name, err = getPoolName(ctx, &pool)
		if err != nil {
			continue
		}

		poolInfo, err := getPoolInfo(ctx, &pool)
		if err != nil {
			continue
		}

		poolDef.Path, err = getPoolPath(ctx, &pool)
		if err != nil {
			continue
		}

		switch poolInfo.State {
		case libvirt.STORAGE_POOL_INACTIVE:
			poolDef.State = "STORAGE_POOL_INACTIVE"
		case libvirt.STORAGE_POOL_BUILDING:
			poolDef.State = "STORAGE_POOL_BUILDING"
		case libvirt.STORAGE_POOL_RUNNING:
			poolDef.State = "STORAGE_POOL_RUNNING"
		case libvirt.STORAGE_POOL_DEGRADED:
			poolDef.State = "STORAGE_POOL_DEGRADED"
		case libvirt.STORAGE_POOL_INACCESSIBLE:
			poolDef.State = "STORAGE_POOL_INACCESSIBLE"
		default:
			poolDef.State = "STORAGE_POOL_UNKNOWN"
		}

		poolDef.Capacity = poolInfo.Capacity
		poolDef.Allocation = poolInfo.Allocation
		poolDef.Available = poolInfo.Available

		poolDef.Autostart, err = isPoolAutostarted(ctx, &pool)
		if err != nil {
			poolDef.Autostart = false
		}

		poolDef.Active, err = isPoolActive(ctx, &pool)
		if err != nil {
			poolDef.Active = false
		}

		poolDef.Persistent, err = isPoolPersistent(ctx, &pool)
		if err != nil {
			poolDef.Persistent = false
		}

		poolDef.VolumesCount, err = countNumOfStorageVolumesInPool(ctx, &pool)
		if err != nil {
			poolDef.VolumesCount = 0
		}

		volumes, err := listPoolVolumes(ctx, &pool)
		if err == nil {
			for _, v := range volumes {
				if strings.Contains(v, "template") {
					poolDef.Templates = append(poolDef.Templates, v)
				}
			}
		}

		r.Pool = append(r.Pool, poolDef)
	}

	return r, nil
}

func getDomainInfoResponse(ctx context.Context, s libvirt.DomainStats) InfoResponse {

	var r InfoResponse
	var err error

	c, err := getConnectFromDomain(ctx, s.Domain)
	defer closeConnection(ctx, c)
	if err != nil || c == nil {
		return InfoResponse{}
	}

	r.Name = getDomainName(ctx, s.Domain)
	r.UUID = getDomainUUID(ctx, s.Domain)
	r.State, r.Reason = getDomainStateStatus(ctx, s.State)
	r.Timestamp = time.Now().Unix()

	r.NodeHost, err = getNodeHostname(ctx, c)
	if err != nil {
		r.NodeHost = unknown
	}

	r.Active = isDomainActive(ctx, s.Domain)
	r.Persistent = isDomainPersistent(ctx, s.Domain)
	r.Updated = isDomainUpdated(ctx, s.Domain)
	r.Autostart = isDomainAutostarted(ctx, s.Domain)

	if s.Balloon != nil {
		r.Mem.Current = s.Balloon.Current
		r.Mem.Maximum = s.Balloon.Maximum
	}

	r.VCPU = make([]vcpuInfo, len(s.Vcpu))
	for i := range s.Vcpu {

		r.VCPU[i].Time = s.Vcpu[i].Time

		switch s.Vcpu[i].State {
		case libvirt.VCPU_OFFLINE:
			r.VCPU[i].State = "VCPU_OFFLINE"
		case libvirt.VCPU_RUNNING:
			r.VCPU[i].State = "VCPU_RUNNING"
		case libvirt.VCPU_BLOCKED:
			r.VCPU[i].State = "VCPU_BLOCKED"
		default:
			r.VCPU[i].State = "VCPU_UNKNOWN"
		}

		r.VCPU[i].Num = uint(i)
	}

	if s.Cpu != nil {
		r.CPU.TotalTime = s.Cpu.Time
		r.CPU.TotalUser = s.Cpu.User
		r.CPU.TotalSystem = s.Cpu.System
		r.CPU.CurrentVCPUs = getDomainCurrentVCPUs(ctx, s.Domain)
		r.CPU.MaximumVCPUs = getDomainMaxVCPUs(ctx, s.Domain)
	}

	r.Net, err = getDomainInterfaceInfo(ctx, s.Domain)
	if err != nil {
		r.Net = []netInfo{}
	}

	r.BlockParams = make([]blockParams, 0, 2)
	r.BlockParams = append(r.BlockParams, getDomainBlkioParams(ctx, s.Domain, libvirt.DOMAIN_AFFECT_CURRENT))
	if r.Persistent && r.Active {
		r.BlockParams = append(r.BlockParams, getDomainBlkioParams(ctx, s.Domain, libvirt.DOMAIN_AFFECT_CONFIG))
	}

	r.Block = make([]blockInfo, len(s.Block))
	for i := range s.Block {
		r.Block[i].Name = s.Block[i].Name
		r.Block[i].Path = s.Block[i].Path
		r.Block[i].BackingIndex = s.Block[i].BackingIndex
		r.Block[i].RdReqs = s.Block[i].RdReqs
		r.Block[i].RdBytes = s.Block[i].RdBytes
		r.Block[i].RdTimes = s.Block[i].RdTimes
		r.Block[i].WrReqs = s.Block[i].WrReqs
		r.Block[i].WrBytes = s.Block[i].WrBytes
		r.Block[i].WrTimes = s.Block[i].WrTimes
		r.Block[i].FlReqs = s.Block[i].FlReqs
		r.Block[i].FlTimes = s.Block[i].FlTimes
		r.Block[i].Errors = s.Block[i].Errors
		r.Block[i].Allocation = s.Block[i].Allocation
		r.Block[i].Capacity = s.Block[i].Capacity
		r.Block[i].Physical = s.Block[i].Physical

		r.Block[i].BlockIO = make([]blockIO, 0, 2)
		r.Block[i].BlockIO = append(r.Block[i].BlockIO, getDomainBlockIoTune(ctx, s.Domain, r.Block[i].Name, libvirt.DOMAIN_AFFECT_CURRENT))
		if r.Persistent && r.Active {
			r.Block[i].BlockIO = append(r.Block[i].BlockIO, getDomainBlockIoTune(ctx, s.Domain, r.Block[i].Name, libvirt.DOMAIN_AFFECT_CONFIG))
		}

		r.Block[i].JobInfo, err = getDomainBlockJobInfo(ctx, s.Domain, r.Block[i].Name)
		if err != nil {
			r.Block[i].JobInfo = blockJobInfo{}
		}
	}

	r.SchedulerInfo = make([]schedulerInfo, 0, 2)

	schedulerInfo, err := getDomainSchedulerInfo(ctx, s.Domain, libvirt.DOMAIN_AFFECT_CURRENT)
	if err == nil {
		r.SchedulerInfo = append(r.SchedulerInfo, schedulerInfo)
	}

	if r.Persistent && r.Active {
		schedulerInfo, err := getDomainSchedulerInfo(ctx, s.Domain, libvirt.DOMAIN_AFFECT_CONFIG)
		if err == nil {
			r.SchedulerInfo = append(r.SchedulerInfo, schedulerInfo)
		}
	}

	r.HypervisorType, err = getDomainHypervisorType(ctx, s.Domain)
	if err != nil {
		r.HypervisorType = unknown
	}

	m, err := getDomainMemoryStats(ctx, s.Domain)
	if err == nil {
		r.Mem.SwapIn = m.SwapIn
		r.Mem.SwapOut = m.SwapOut
		r.Mem.MajorFault = m.MajorFault
		r.Mem.MinorFault = m.MinorFault
		r.Mem.Unused = m.Unused
		r.Mem.Available = m.Available
		r.Mem.Usable = m.Usable
		r.Mem.Used = m.Available - m.Unused
		r.Mem.LastUpdate = m.LastUpdate
		r.Mem.Rss = m.Rss

		period, err := getDomainMemoryStatsPeriod(ctx, s.Domain)
		if err != nil {
			r.Mem.Period = 0
		}

		r.Mem.Period = period
	}

	r.Security = getDomainSecurityStatus(ctx, s.Domain)

	r.SnapshotCount, err = countDomainSnapshotsWithFlags(ctx, s.Domain, libvirt.DomainSnapshotListFlags(0))
	if err != nil {
		r.SnapshotCount = 0
	}

	r.SnapshotInfo = listDomainSnapshots(ctx, s.Domain)

	return r
}

func getQemuAgentInfoResponse(ctx context.Context, d *libvirt.Domain) QemuAgentResponse {

	var r QemuAgentResponse

	r.Available = isGuestAgentAvailable(ctx, d)
	if r.Available {

		version, err := getGuestAgentVersion(ctx, d)
		if err == nil {
			r.AgentVersion = version
		}

		r.Time, err = getGuestTime(ctx, d)
		if err != nil {
			r.Time = 0
		}

		fsInfo, err := getGuestFSInfo(ctx, d)
		if err != nil {
			r.FSInfo = []domainFSInfo{}
		}

		for _, v := range fsInfo {
			r.FSInfo = append(r.FSInfo, domainFSInfo{
				MountPoint: v.MountPoint,
				Name:       v.Name,
				FSType:     v.FSType,
				DevAlias:   v.DevAlias,
			})
		}

		osInfo, err := getGuestOsInfo(ctx, d)
		if err == nil {

			var os guestOsInfo

			os.ID = osInfo.ID
			os.KernelRelease = osInfo.KernelRelease
			os.KernelVersion = osInfo.KernelVersion
			os.Machine = osInfo.Machine
			os.Name = osInfo.Name
			os.PrettyName = osInfo.PrettyName
			os.Version = osInfo.Version
			os.VersionID = osInfo.VersionID

			r.OSInfo = os
		}

		timezone, err := getGuestTimezone(ctx, d)
		if err == nil {
			r.Timezone = timezone
		}

		hostname, err := getGuestHostname(ctx, d)
		if err == nil {
			r.Hostname = hostname
		}

		r.Network, err = getGuestNetworkInfo(ctx, d)
		if err != nil {
			r.Network = []guestNetwork{}
		}

		r.LoadAverege, err = getGuestLinuxLA(ctx, d)
		if err != nil {
			r.LoadAverege = guestLA{}
		}

		r.Users, err = getGuestUnixUsers(ctx, d)
		if err != nil {
			r.Users = []guestUser{}
		}

		r.Uptime, err = getGuestLinuxUptime(ctx, d)
		if err != nil {
			r.Uptime = guestUptime{}
		}

	}

	return r
}

func getDomainsInfoResponse(ctx context.Context, stats []libvirt.DomainStats, length int) []InfoResponse {

	r := make([]InfoResponse, 0, length) // 256 is actual libvirt limit for domains on single hypervisor

	for _, stat := range stats {
		r = append(r, getDomainInfoResponse(ctx, stat))
		freeDomain(ctx, stat.Domain)
	}

	return r
}

func getDomainStateStatus(ctx context.Context, s *libvirt.DomainStatsState) (string, string) {

	if s == nil {
		return "", ""
	}

	const domainBlockedUnknown = "DOMAIN_BLOCKED_UNKNOWN"
	const domainStateUnknown = "DOMAIN_STATE_UNKNOWN"
	const domainReasonUnknown = "DOMAIN_REASON_UNKNOWN"
	const domainNoStateUnknown = "DOMAIN_NOSTATE_UNKNOWN"
	const domainRunningUnknown = "DOMAIN_RUNNING_UNKNOWN"
	const domainPmSuspendedUnknown = "DOMAIN_PMSUSPENDED_UNKNOWN"
	const domainPausedUnknown = "DOMAIN_PAUSED_UNKNOWN"
	const domainCrashedUnknown = "DOMAIN_CRASHED_UNKNOWN"
	const domainShutoffUnknown = "DOMAIN_SHUTOFF_UNKNOWN"
	const domainShutdownUnknown = "DOMAIN_SHUTDOWN_UNKNOWN"

	var strState, strReason string
	structState := s.State

	if s.StateSet {
		switch structState {

		case libvirt.DOMAIN_NOSTATE:
			strState = "DOMAIN_NOSTATE"
			if s.ReasonSet {
				structReason := libvirt.DomainNostateReason(s.Reason)
				switch structReason {
				case libvirt.DOMAIN_NOSTATE_UNKNOWN:
					strReason = domainNoStateUnknown
				default:
					strReason = domainNoStateUnknown
				}
			} else {
				strReason = domainNoStateUnknown
			}

		case libvirt.DOMAIN_RUNNING:
			strState = "DOMAIN_RUNNING"
			if s.ReasonSet {
				structReason := libvirt.DomainRunningReason(s.Reason)
				switch structReason {
				case libvirt.DOMAIN_RUNNING_UNKNOWN:
					strReason = domainRunningUnknown
				case libvirt.DOMAIN_RUNNING_BOOTED:
					strReason = "DOMAIN_RUNNING_BOOTED"
				case libvirt.DOMAIN_RUNNING_MIGRATED:
					strReason = "DOMAIN_RUNNING_MIGRATED"
				case libvirt.DOMAIN_RUNNING_RESTORED:
					strReason = "DOMAIN_RUNNING_RESTORED"
				case libvirt.DOMAIN_RUNNING_FROM_SNAPSHOT:
					strReason = "DOMAIN_RUNNING_FROM_SNAPSHOT"
				case libvirt.DOMAIN_RUNNING_UNPAUSED:
					strReason = "DOMAIN_RUNNING_UNPAUSED"
				case libvirt.DOMAIN_RUNNING_MIGRATION_CANCELED:
					strReason = "DOMAIN_RUNNING_MIGRATION_CANCELED"
				case libvirt.DOMAIN_RUNNING_SAVE_CANCELED:
					strReason = "DOMAIN_RUNNING_SAVE_CANCELED"
				case libvirt.DOMAIN_RUNNING_WAKEUP:
					strReason = "DOMAIN_RUNNING_WAKEUP"
				case libvirt.DOMAIN_RUNNING_CRASHED:
					strReason = "DOMAIN_RUNNING_CRASHED"
				case libvirt.DOMAIN_RUNNING_POSTCOPY:
					strReason = "DOMAIN_RUNNING_POSTCOPY"
				default:
					strReason = domainRunningUnknown
				}
			} else {
				strReason = domainRunningUnknown
			}

		case libvirt.DOMAIN_BLOCKED:
			strState = "DOMAIN_BLOCKED"
			if s.ReasonSet {
				structReason := libvirt.DomainBlockedReason(s.Reason)
				switch structReason {
				case libvirt.DOMAIN_BLOCKED_UNKNOWN:
					strReason = domainBlockedUnknown
				default:
					strReason = domainBlockedUnknown
				}
			} else {
				strReason = domainBlockedUnknown
			}

		case libvirt.DOMAIN_PAUSED:
			strState = "DOMAIN_PAUSED"
			if s.ReasonSet {
				structReason := libvirt.DomainPausedReason(s.Reason)
				switch structReason {
				case libvirt.DOMAIN_PAUSED_UNKNOWN:
					strReason = domainPausedUnknown
				case libvirt.DOMAIN_PAUSED_USER:
					strReason = "DOMAIN_PAUSED_USER"
				case libvirt.DOMAIN_PAUSED_MIGRATION:
					strReason = "DOMAIN_PAUSED_MIGRATION"
				case libvirt.DOMAIN_PAUSED_SAVE:
					strReason = "DOMAIN_PAUSED_SAVE"
				case libvirt.DOMAIN_PAUSED_DUMP:
					strReason = "DOMAIN_PAUSED_DUMP"
				case libvirt.DOMAIN_PAUSED_IOERROR:
					strReason = "DOMAIN_PAUSED_IOERROR"
				case libvirt.DOMAIN_PAUSED_WATCHDOG:
					strReason = "DOMAIN_PAUSED_WATCHDOG"
				case libvirt.DOMAIN_PAUSED_FROM_SNAPSHOT:
					strReason = "DOMAIN_PAUSED_FROM_SNAPSHOT"
				case libvirt.DOMAIN_PAUSED_SHUTTING_DOWN:
					strReason = "DOMAIN_PAUSED_SHUTTING_DOWN"
				case libvirt.DOMAIN_PAUSED_SNAPSHOT:
					strReason = "DOMAIN_PAUSED_SNAPSHOT"
				case libvirt.DOMAIN_PAUSED_CRASHED:
					strReason = "DOMAIN_PAUSED_CRASHED"
				case libvirt.DOMAIN_PAUSED_STARTING_UP:
					strReason = "DOMAIN_PAUSED_STARTING_UP"
				case libvirt.DOMAIN_PAUSED_POSTCOPY:
					strReason = "DOMAIN_PAUSED_POSTCOPY"
				case libvirt.DOMAIN_PAUSED_POSTCOPY_FAILED:
					strReason = "DOMAIN_PAUSED_POSTCOPY_FAILED"
				default:
					strReason = domainPausedUnknown
				}
			} else {
				strReason = domainPausedUnknown
			}

		case libvirt.DOMAIN_SHUTDOWN:
			strState = "DOMAIN_SHUTDOWN"
			if s.ReasonSet {
				structReason := libvirt.DomainShutdownReason(s.Reason)
				switch structReason {
				case libvirt.DOMAIN_SHUTDOWN_UNKNOWN:
					strReason = domainShutdownUnknown
				case libvirt.DOMAIN_SHUTDOWN_USER:
					strReason = "DOMAIN_SHUTDOWN_USER"
				default:
					strReason = domainShutdownUnknown
				}
			} else {
				strReason = domainShutdownUnknown
			}

		case libvirt.DOMAIN_CRASHED:
			strState = "DOMAIN_CRASHED"
			if s.ReasonSet {
				structReason := libvirt.DomainCrashedReason(s.Reason)
				switch structReason {
				case libvirt.DOMAIN_CRASHED_UNKNOWN:
					strReason = domainCrashedUnknown
				case libvirt.DOMAIN_CRASHED_PANICKED:
					strReason = "DOMAIN_CRASHED_PANICKED"
				default:
					strReason = domainCrashedUnknown
				}
			} else {
				strReason = domainCrashedUnknown
			}

		case libvirt.DOMAIN_PMSUSPENDED:
			strState = "DOMAIN_PMSUSPENDED"
			if s.ReasonSet {
				structReason := libvirt.DomainPMSuspendedReason(s.Reason)
				switch structReason {
				case libvirt.DOMAIN_PMSUSPENDED_UNKNOWN:
					strReason = domainPmSuspendedUnknown
				default:
					strReason = domainPmSuspendedUnknown
				}
			} else {
				strReason = domainPmSuspendedUnknown
			}

		case libvirt.DOMAIN_SHUTOFF:
			strState = "DOMAIN_SHUTOFF"
			if s.ReasonSet {
				structReason := libvirt.DomainShutoffReason(s.Reason)
				switch structReason {
				case libvirt.DOMAIN_SHUTOFF_UNKNOWN:
					strReason = domainShutoffUnknown
				case libvirt.DOMAIN_SHUTOFF_SHUTDOWN:
					strReason = "DOMAIN_SHUTOFF_SHUTDOWN"
				case libvirt.DOMAIN_SHUTOFF_DESTROYED:
					strReason = "DOMAIN_SHUTOFF_DESTROYED"
				case libvirt.DOMAIN_SHUTOFF_CRASHED:
					strReason = "DOMAIN_SHUTOFF_CRASHED"
				case libvirt.DOMAIN_SHUTOFF_MIGRATED:
					strReason = "DOMAIN_SHUTOFF_MIGRATED"
				case libvirt.DOMAIN_SHUTOFF_SAVED:
					strReason = "DOMAIN_SHUTOFF_SAVED"
				case libvirt.DOMAIN_SHUTOFF_FAILED:
					strReason = "DOMAIN_SHUTOFF_FAILED"
				case libvirt.DOMAIN_SHUTOFF_FROM_SNAPSHOT:
					strReason = "DOMAIN_SHUTOFF_FROM_SNAPSHOT"
				default:
					strReason = domainShutoffUnknown
				}
			} else {
				strReason = domainShutoffUnknown
			}

		default:
			strState = domainStateUnknown
			strReason = domainReasonUnknown
		}

	} else {
		strState = domainStateUnknown
		strReason = domainReasonUnknown
	}

	return strState, strReason
}
