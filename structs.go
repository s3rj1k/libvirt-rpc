package main

// https://libvirt.org/html/libvirt-libvirt-domain.html#virConnectGetAllDomainStats
type cpuInfo struct {
	TotalTime    uint64 `json:"TotalTime"`   // nanoseconds
	TotalUser    uint64 `json:"TotalUser"`   // nanoseconds
	TotalSystem  uint64 `json:"TotalSystem"` // nanoseconds
	CurrentVCPUs uint64 `json:"CurrentVCPUs"`
	MaximumVCPUs uint64 `json:"MaximumVCPUs"`
}

// https://libvirt.org/html/libvirt-libvirt-domain.html#virDomainMemoryStats
type memInfo struct {
	Current    uint64 `json:"Current"` // KiB
	Maximum    uint64 `json:"Maximum"` // KiB
	SwapIn     uint64 `json:"SwapIn"`  // KiB
	SwapOut    uint64 `json:"SwapOut"` // KiB
	MajorFault uint64 `json:"MajorFault"`
	MinorFault uint64 `json:"MinorFault"`
	Unused     uint64 `json:"Unused"`    // KiB
	Available  uint64 `json:"Available"` // KiB
	Usable     uint64 `json:"Usable"`    // KiB
	Used       uint64 `json:"Used"`      // KiB
	Rss        uint64 `json:"Rss"`       // KiB
	LastUpdate uint64 `json:"LastUpdate"`
	Period     uint   `json:"Period"` // seconds
}

type vcpuInfo struct {
	Num   uint   `json:"Num"`
	State string `json:"State"`
	Time  uint64 `json:"Time"` // nanoseconds
}

type netInfo struct {
	MAC      string      `json:"MAC"`
	PVID     string      `json:"PVID"`
	PFName   string      `json:"PFName"`
	VFName   string      `json:"VFName"`
	Network  string      `json:"Network"`
	PCI      netPCI      `json:"PCI"`
	Metadata netMetadata `json:"Metadata"`
	Desc     string      `json:"Desc"`
}

type netPCI struct {
	VFaddr string `json:"VFaddr"`
	PFaddr string `json:"PFaddr"`
	VFName string `json:"VFName"`
	PFName string `json:"PFName"`
}

type netMetadata struct {
	MaxTxRate uint   `json:"MaxTxRate"` // Mbps
	QoS       uint   `json:"QoS"`
	Trust     string `json:"Trust"`
	SpoofChk  string `json:"SpoofChk"`
	QueryRss  string `json:"QueryRss"`
}

// https://libvirt.org/html/libvirt-libvirt-domain.html#virConnectGetAllDomainStats
type blockInfo struct {
	Name         string       `json:"Name"`
	BackingIndex uint         `json:"BackingIndex"`
	Path         string       `json:"Path"`
	RdReqs       uint64       `json:"RdReqs"`
	RdBytes      uint64       `json:"RdBytes"` // bytes
	RdTimes      uint64       `json:"RdTimes"` // nanoseconds
	WrReqs       uint64       `json:"WrReqs"`
	WrBytes      uint64       `json:"WrBytes"` // bytes
	WrTimes      uint64       `json:"WrTimes"` // nanoseconds
	FlReqs       uint64       `json:"FlReqs"`
	FlTimes      uint64       `json:"FlTimes"`    // nanoseconds
	Errors       uint64       `json:"Errors"`     // Xen
	Allocation   uint64       `json:"Allocation"` // bytes
	Capacity     uint64       `json:"Capacity"`   // bytes
	Physical     uint64       `json:"Physical"`   // bytes
	BlockIO      []blockIO    `json:"BlockIO"`
	JobInfo      blockJobInfo `json:"JobInfo"`
}

type blockJobInfo struct {
	Type      string `json:"Type"`
	Bandwidth uint64 `json:"Bandwidth"`
	Cur       uint64 `json:"Cur"`
	End       uint64 `json:"End"`
}

type snapshotInfo struct {
	Name          string   `json:"Name"`
	Parent        string   `json:"Parent"`
	ChildrenCount int      `json:"ChildrenCount"`
	IsCurrent     bool     `json:"IsCurrent"`
	IsInternal    bool     `json:"IsInternal"`
	IsExternal    bool     `json:"IsExternal"`
	IsDiskOnly    bool     `json:"IsDiskOnly"`
	WasActive     bool     `json:"WasActive"`
	WasInactive   bool     `json:"WasInactive"`
	HasMetadata   bool     `json:"HasMetadata"`
	HasNoMetadata bool     `json:"HasNoMetadata"`
	HasChildren   bool     `json:"HasChildren"`
	HasNoChildren bool     `json:"HasNoChildren"`
	HasNoParents  bool     `json:"HasNoParents"`
	Error         bool     `json:"Error"`
	ErrorMessage  []string `json:"ErrorMessage"`
}

// virsh help blkdeviotune
type blockIO struct {
	ModificationImpact     string `json:"ModificationImpact"`
	ReadBytesSec           uint64 `json:"ReadBytesSec"`           // bytes/s
	ReadBytesSecMax        uint64 `json:"ReadBytesSecMax"`        // bytes/s
	ReadBytesSecMaxLength  uint64 `json:"ReadBytesSecMaxLength"`  // seconds
	ReadIopsSec            uint64 `json:"ReadIopsSec"`            // iops
	ReadIopsSecMax         uint64 `json:"ReadIopsSecMax"`         // iops
	ReadIopsSecMaxLength   uint64 `json:"ReadIopsSecMaxLength"`   // seconds
	SizeIopsSec            uint64 `json:"SizeIopsSec"`            // iops
	TotalBytesSec          uint64 `json:"TotalBytesSec"`          // bytes/s
	TotalBytesSecMax       uint64 `json:"TotalBytesSecMax"`       // bytes/s
	TotalBytesSecMaxLength uint64 `json:"TotalBytesSecMaxLength"` // seconds
	TotalIopsSec           uint64 `json:"TotalIopsSec"`           // iops
	TotalIopsSecMax        uint64 `json:"TotalIopsSecMax"`        // iops
	TotalIopsSecMaxLength  uint64 `json:"TotalIopsSecMaxLength"`  // seconds
	WriteBytesSec          uint64 `json:"WriteBytesSec"`          // bytes/s
	WriteBytesSecMax       uint64 `json:"WriteBytesSecMax"`       // bytes/s
	WriteBytesSecMaxLength uint64 `json:"WriteBytesSecMaxLength"` // seconds
	WriteIopsSec           uint64 `json:"WriteIopsSec"`           // iops
	WriteIopsSecMax        uint64 `json:"WriteIopsSecMax"`        // iops
	WriteIopsSecMaxLength  uint64 `json:"WriteIopsSecMaxLength"`  // seconds
	GroupName              string `json:"GroupName"`
}

// virsh help blkiotune
type blockParams struct {
	ModificationImpact string `json:"ModificationImpact"`
	Weight             uint   `json:"Weight"`
	DeviceWeight       string `json:"DeviceWeight"`
	DeviceReadIops     string `json:"DeviceReadIops"`  // iops
	DeviceWriteIops    string `json:"DeviceWriteIops"` // iops
	DeviceReadBps      string `json:"DeviceReadBps"`   // bytes/s
	DeviceWriteBps     string `json:"DeviceWriteBps"`  // bytes/s
}

// https://libvirt.org/formatdomain.html#elementsCPUTuning
type schedulerInfo struct {
	ModificationImpact string `json:"ModificationImpact"`
	Type               string `json:"Type"`
	CPUShares          uint64 `json:"CPUShares"`      // QEMU
	GlobalPeriod       uint64 `json:"GlobalPeriod"`   // microseconds
	GlobalQuota        int64  `json:"GlobalQuota"`    // microseconds
	VcpuPeriod         uint64 `json:"VcpuPeriod"`     // microseconds
	VcpuQuota          int64  `json:"VcpuQuota"`      // microseconds
	EmulatorPeriod     uint64 `json:"EmulatorPeriod"` // microseconds
	EmulatorQuota      int64  `json:"EmulatorQuota"`  // microseconds
	IothreadPeriod     uint64 `json:"IothreadPeriod"` // microseconds
	IothreadQuota      int64  `json:"IothreadQuota"`  // microseconds
	Weight             uint   `json:"Weight"`         // XEN
	Cap                uint   `json:"Cap"`            // XEN
	Reservation        int64  `json:"Reservation"`    // XEN
	Limit              int64  `json:"Limit"`          // XEN
	Shares             int    `json:"Shares"`         // XEN
}

// InfoResponse - struct for JRPC Info function
type InfoResponse struct {
	Name           string          `json:"Name"`
	UUID           string          `json:"UUID"`
	Timestamp      int64           `json:"Timestamp"`
	Active         bool            `json:"Active"`
	Persistent     bool            `json:"Persistent"`
	Updated        bool            `json:"Updated"`
	Autostart      bool            `json:"Autostart"`
	State          string          `json:"State"`
	Reason         string          `json:"Reason"`
	NodeHost       string          `json:"NodeFQDN"`
	HypervisorType string          `json:"HypervisorType"`
	Security       string          `json:"Security"`
	SchedulerInfo  []schedulerInfo `json:"SchedulerInfo"`
	CPU            cpuInfo         `json:"CPU"`
	VCPU           []vcpuInfo      `json:"VCPU"`
	Mem            memInfo         `json:"Mem"`
	Net            []netInfo       `json:"Net"`
	BlockParams    []blockParams   `json:"BlockParams"`
	Block          []blockInfo     `json:"Block"`
	SnapshotCount  int             `json:"SnapshotCount"`
	SnapshotInfo   []snapshotInfo  `json:"SnapshotInfo"`
}

type guestNetworkIPAddress struct {
	IPAddressType string `json:"Type"`
	IPAddress     string `json:"IP"`
	Prefix        int    `json:"Prefix"`
}

type guestNetworkStatistics struct {
	RxBytes   int `json:"RxBytes"`
	RxDropped int `json:"RxDropped"`
	RxErrs    int `json:"RxErrs"`
	RxPackets int `json:"RxPackets"`
	TxBytes   int `json:"TxBytes"`
	TxDropped int `json:"TxBropped"`
	TxErrs    int `json:"TxErrs"`
	TxPackets int `json:"TxPackets"`
}

type guestNetwork struct {
	Name            string                  `json:"Name"`
	IPAddresses     []guestNetworkIPAddress `json:"IP"`
	Statistics      guestNetworkStatistics  `json:"Statistics"`
	HardwareAddress string                  `json:"MAC"`
}

type guestLA struct {
	OneMinutes       float64 `json:"OneMinutes"`
	FiveMinutes      float64 `json:"FiveMinutes"`
	TenMinutes       float64 `json:"TenMinutes"`
	CurrentProcesses uint    `json:"CurrentProcesses"`
	TotalProcesses   uint    `json:"TotalProcesses"`
}

type guestUser struct {
	Name    string `json:"Name"`
	HomeDir string `json:"HomeDir"`
	Shell   string `json:"Shell"`
}

type guestUptime struct {
	Up   float64 `json:"Up"`   // seconds
	Idle float64 `json:"Idle"` // seconds
}

type guestOsInfo struct {
	ID            string `json:"ID"`
	KernelRelease string `json:"KernelRelease"`
	KernelVersion string `json:"KernelVersion"`
	Machine       string `json:"Machine"`
	Name          string `json:"Name"`
	PrettyName    string `json:"PrettyName"`
	Version       string `json:"Version"`
	VersionID     string `json:"VersionID"`
}

type domainFSInfo struct {
	MountPoint string   `json:"MountPoint"`
	Name       string   `json:"Name"`
	FSType     string   `json:"FSType"`
	DevAlias   []string `json:"DevAlias"`
}

// QemuAgentResponse - struct for JRPC QemuAgentInfo function
type QemuAgentResponse struct {
	Available    bool           `json:"Available"`
	AgentVersion string         `json:"AgentVersion"`
	Time         int64          `json:"Time"`
	Timezone     string         `json:"Timezone"`
	Hostname     string         `json:"Hostname"`
	OSInfo       guestOsInfo    `json:"OSInfo"`
	LoadAverege  guestLA        `json:"LoadAverege"`
	Uptime       guestUptime    `json:"Uptime"`
	Users        []guestUser    `json:"Users"`
	FSInfo       []domainFSInfo `json:"FSInfo"`
	Network      []guestNetwork `json:"Network"`
}

type nodeInfo struct {
	Model   string `json:"Model"`
	Memory  uint64 `json:"Memory"`
	Cpus    uint   `json:"Cpus"`
	MHz     uint   `json:"MHz"`
	Nodes   uint32 `json:"Nodes"`
	Sockets uint32 `json:"Sockets"`
	Cores   uint32 `json:"Cores"`
	Threads uint32 `json:"Threads"`
}

// NodeInfoResponse - struct for JRPC HypervisorInfo function
type NodeInfoResponse struct {
	Hostname           string          `json:"Hostname"`
	Timestamp          int64           `json:"Timestamp"`
	Uptime             uint64          `json:"Uptime"` // nanoseconds
	LibvirtVersion     uint32          `json:"LibvirtVersion"`
	VCPUsCount         uint32          `json:"VCPUsCount"`
	ActiveNetworkCount uint32          `json:"ActiveNetworkCount"`
	ActiveDomainCount  uint32          `json:"ActiveDomainCount"`
	HardwareInfo       nodeInfo        `json:"HardwareInfo"`
	CPUStats           nodeCPUStats    `json:"CPUStats"`
	MemoryStats        nodeMemoryStats `json:"MemoryStats"`
	Network            []nodeNetwork   `json:"Network"`
	Pool               []nodePool      `json:"Pool"`
}

type nodeNetwork struct {
	Name         string `json:"Name"`
	UsedVFs      int    `json:"UsedVFs"`
	AvaliableVFs int    `json:"AvaliableVFs"`
	TotalVFs     int    `json:"TotalVFs"`
}

// https://libvirt.org/html/libvirt-libvirt-host.html#virNodeGetCPUStats
type nodeCPUStats struct {
	Kernel      uint64 `json:"Kernel"`      // nanoseconds
	User        uint64 `json:"User"`        // nanoseconds
	Idle        uint64 `json:"Idle"`        // nanoseconds
	Iowait      uint64 `json:"Iowait"`      // nanoseconds
	Interrupt   uint64 `json:"Interrupt"`   // nanoseconds
	Utilization uint64 `json:"Utilization"` // %
}

type nodeMemoryStats struct {
	Total      uint64 `json:"Total"`      // KiB
	Available  uint64 `json:"Available"`  // KiB
	Used       uint64 `json:"Used"`       // KiB
	Free       uint64 `json:"Free"`       // KiB
	Cached     uint64 `json:"Cached"`     // KiB
	Buffers    uint64 `json:"Buffers"`    // KiB
	SwapTotal  uint64 `json:"SwapTotal"`  // KiB
	SwapFree   uint64 `json:"SwapFree"`   // KiB
	SwapCached uint64 `json:"SwapCached"` // KiB
}

type nodePool struct {
	Name         string   `json:"Name"`
	State        string   `json:"State"`
	Active       bool     `json:"Active"`
	Persistent   bool     `json:"Persistent"`
	Autostart    bool     `json:"Autostart"`
	Capacity     uint64   `json:"Capacity"`   // bytes
	Allocation   uint64   `json:"Allocation"` // bytes
	Available    uint64   `json:"Available"`  // bytes
	Path         string   `json:"Path"`
	VolumesCount int      `json:"VolumesCount"`
	Templates    []string `json:"Templates"`
}
