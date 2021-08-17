package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/libvirt/libvirt-go"
)

// https://github.com/qemu/qemu/blob/master/qga/qapi-schema.json

type guestOsInfoUnmarshal struct {
	Name          string `json:"name,omitempty"`
	KernelRelease string `json:"kernel-release,omitempty"`
	Version       string `json:"version,omitempty"`
	PrettyName    string `json:"pretty-name,omitempty"`
	VersionID     string `json:"version-id,omitempty"`
	KernelVersion string `json:"kernel-version,omitempty"`
	Machine       string `json:"machine,omitempty"`
	ID            string `json:"id,omitempty"`
}

func guestFileOpen(ctx context.Context, d *libvirt.Domain, path string, mode string) (int, error) {

	id := getReqIDFromContext(ctx)

	type fd struct {
		Return int `json:"return"`
	}

	var err error
	var s fd

	cmd := fmt.Sprintf("{\"execute\": \"guest-file-open\",\"arguments\": {\"path\": \"%s\",\"mode\": \"%s\"}}", path, mode)

	rawJSON, err := qemuAgentCommand(ctx, d, cmd)
	if err != nil {
		return 0, err
	}

	err = json.Unmarshal([]byte(rawJSON), &s)
	if err != nil {
		fail.Printf("%sfailed to unmarshal Qemu-Agent JSON output: %s", id, err.Error())
		return 0, err
	}

	info.Printf("%sunmarshaled Qemu-Agent JSON output\n", id)
	return s.Return, nil
}

func guestFileClose(ctx context.Context, d *libvirt.Domain, fd int) error {

	id := getReqIDFromContext(ctx)

	cmd := fmt.Sprintf("{\"execute\": \"guest-file-close\",\"arguments\": {\"handle\": %d}}", fd)

	_, err := qemuAgentCommand(ctx, d, cmd)

	if err != nil {
		fail.Printf("%sfailed to close file handle inside Guest: %s", id, err.Error())
	} else {
		info.Printf("%sclosed file handle inside Guest\n", id)
	}

	return err
}

func guestFileRead(ctx context.Context, d *libvirt.Domain, fd int) (string, error) {

	id := getReqIDFromContext(ctx)

	type fileReadRaw struct {
		Return struct {
			Count  int    `json:"count"`
			BufB64 string `json:"buf-b64"`
			EOF    bool   `json:"eof"`
		} `json:"return"`
	}

	var err error
	var s fileReadRaw

	cmd := fmt.Sprintf("{\"execute\": \"guest-file-read\",\"arguments\": {\"handle\": %d}}", fd)

	rawJSON, err := qemuAgentCommand(ctx, d, cmd)
	if err != nil {
		return "", err
	}

	err = json.Unmarshal([]byte(rawJSON), &s)
	if err != nil {
		fail.Printf("%sfailed to unmarshal Qemu-Agent JSON output: %s", id, err.Error())
		return "", err
	}

	if !s.Return.EOF {
		return "", errors.New("not reached EOF")
	}

	decoded, err := base64.StdEncoding.DecodeString(s.Return.BufB64)
	if err != nil {
		fail.Printf("%sfailed to decode base64 payload: %s", id, err.Error())
		return "", err
	}

	if len(decoded) != s.Return.Count {
		return "", errors.New("decoded base64 payload does not match original string size")
	}

	return string(decoded[:]), nil
}

func getGuestLinuxLA(ctx context.Context, d *libvirt.Domain) (guestLA, error) {

	id := getReqIDFromContext(ctx)

	fd, err := guestFileOpen(ctx, d, "/proc/loadavg", "r")
	if err != nil {
		return guestLA{}, err
	}

	out, err := guestFileRead(ctx, d, fd)

	defer func() {
		err = guestFileClose(ctx, d, fd)
		if err != nil {
			fail.Printf("%sfailed to defer: %s", id, err.Error())
		}
	}()

	if err != nil {
		return guestLA{}, err
	}

	var loadAvg guestLA

	outSplit := strings.Split(strings.TrimSpace(out), " ")

	if len(outSplit) < 5 {
		return guestLA{}, errors.New("unknown /proc/loadavg file format")
	}

	var f float64

	f, err = strconv.ParseFloat(outSplit[0], 64)
	if err == nil {
		loadAvg.OneMinutes = f
	}

	f, err = strconv.ParseFloat(outSplit[1], 64)
	if err == nil {
		loadAvg.FiveMinutes = f
	}

	f, err = strconv.ParseFloat(outSplit[2], 64)
	if err == nil {
		loadAvg.TenMinutes = f
	}

	procSplit := strings.Split(outSplit[3], "/")

	if len(procSplit) == 2 {

		var u uint64

		u, err = strconv.ParseUint(procSplit[0], 10, 64)
		if err == nil {
			loadAvg.CurrentProcesses = uint(u)
		}

		u, err = strconv.ParseUint(procSplit[1], 10, 64)
		if err == nil {
			loadAvg.TotalProcesses = uint(u)
		}

	}

	return loadAvg, nil
}

func getGuestLinuxUptime(ctx context.Context, d *libvirt.Domain) (guestUptime, error) {

	id := getReqIDFromContext(ctx)

	var outStruct guestUptime

	fd, err := guestFileOpen(ctx, d, "/proc/uptime", "r")
	if err != nil {
		return guestUptime{}, err
	}

	out, err := guestFileRead(ctx, d, fd)

	defer func() {
		err = guestFileClose(ctx, d, fd)
		if err != nil {
			fail.Printf("%sfailed to defer: %s", id, err.Error())
		}
	}()

	if err != nil {
		return guestUptime{}, err
	}

	outSplit := strings.Split(strings.TrimSpace(out), " ")

	if len(outSplit) < 2 {
		return guestUptime{}, errors.New("unknown /proc/uptime file format")
	}

	var uptime, idle float64

	uptime, err = strconv.ParseFloat(outSplit[0], 64)
	if err != nil {
		return guestUptime{}, err
	}

	idle, err = strconv.ParseFloat(outSplit[1], 64)
	if err != nil {
		return guestUptime{}, err
	}

	outStruct.Up = uptime
	outStruct.Idle = idle

	return outStruct, nil
}

func getGuestUnixUsers(ctx context.Context, d *libvirt.Domain) ([]guestUser, error) {

	id := getReqIDFromContext(ctx)

	fd, err := guestFileOpen(ctx, d, "/etc/passwd", "r")
	if err != nil {
		return []guestUser{}, err
	}

	out, err := guestFileRead(ctx, d, fd)

	defer func() {
		err = guestFileClose(ctx, d, fd)
		if err != nil {
			fail.Printf("%sfailed to defer: %s", id, err.Error())
		}
	}()

	if err != nil {
		return []guestUser{}, err
	}

	outSplitNewLine := strings.Split(strings.TrimSpace(out), "\n")
	users := make([]guestUser, 0, len(outSplitNewLine))

	for _, ln := range outSplitNewLine {

		userLineSplit := strings.Split(ln, ":")

		if len(userLineSplit) == 7 {

			uid, err := strconv.ParseUint(userLineSplit[2], 10, 64)
			if err == nil {

				if uid >= 1000 && uid != 65534 {

					var user guestUser

					user.Name = userLineSplit[0]
					user.HomeDir = userLineSplit[5]
					user.Shell = userLineSplit[6]

					users = append(users, user)
				}
			}
		}
	}

	return users, nil
}

func getGuestOsInfo(ctx context.Context, d *libvirt.Domain) (guestOsInfoUnmarshal, error) {

	id := getReqIDFromContext(ctx)

	type guestOsInfoRaw struct {
		Return guestOsInfoUnmarshal `json:"return"`
	}

	var err error
	var s guestOsInfoRaw

	rawJSON, err := qemuAgentCommand(ctx, d, "{\"execute\":\"guest-get-osinfo\"}")
	if err != nil {
		return guestOsInfoUnmarshal{}, err
	}

	err = json.Unmarshal([]byte(rawJSON), &s)
	if err != nil {
		fail.Printf("%sfailed to unmarshal Qemu-Agent JSON output: %s", id, err.Error())
		return guestOsInfoUnmarshal{}, err
	}

	return s.Return, nil
}

func getGuestTimezone(ctx context.Context, d *libvirt.Domain) (string, error) {

	id := getReqIDFromContext(ctx)

	type timeZoneRaw struct {
		Return struct {
			Zone   string `json:"zone,omitempty"`
			Offset int    `json:"offset,omitempty"`
		} `json:"return"`
	}

	var err error
	var s timeZoneRaw
	var separator string

	rawJSON, err := qemuAgentCommand(ctx, d, "{\"execute\":\"guest-get-timezone\"}")
	if err != nil {
		return "", err
	}

	err = json.Unmarshal([]byte(rawJSON), &s)
	if err != nil {
		fail.Printf("%sfailed to unmarshal Qemu-Agent JSON output: %s", id, err.Error())
		return "", err
	}

	if s.Return.Offset == 0 {
		separator = "+"
	} else {
		separator = ""
	}

	return fmt.Sprintf("%s%s%d", s.Return.Zone, separator, s.Return.Offset), nil
}

func getGuestHostname(ctx context.Context, d *libvirt.Domain) (string, error) {

	id := getReqIDFromContext(ctx)

	type hostnameRaw struct {
		Return struct {
			HostName string `json:"host-name,omitempty"`
		} `json:"return"`
	}

	var err error
	var s hostnameRaw

	rawJSON, err := qemuAgentCommand(ctx, d, "{\"execute\":\"guest-get-host-name\"}")
	if err != nil {
		return "", err
	}

	err = json.Unmarshal([]byte(rawJSON), &s)
	if err != nil {
		fail.Printf("%sfailed to unmarshal Qemu-Agent JSON output: %s", id, err.Error())
		return "", err
	}

	return s.Return.HostName, nil
}

func getGuestNetworkInfo(ctx context.Context, d *libvirt.Domain) ([]guestNetwork, error) {

	id := getReqIDFromContext(ctx)

	type networkRaw struct {
		Return []struct {
			Name        string `json:"name,omitempty"`
			IPAddresses []struct {
				IPAddressType string `json:"ip-address-type,omitempty"`
				IPAddress     string `json:"ip-address,omitempty"`
				Prefix        int    `json:"prefix,omitempty"`
			} `json:"ip-addresses,omitempty"`
			Statistics struct {
				RxBytes   int `json:"rx-bytes,omitempty"`
				RxDropped int `json:"rx-dropped,omitempty"`
				RxErrs    int `json:"rx-errs,omitempty"`
				RxPackets int `json:"rx-packets,omitempty"`
				TxBytes   int `json:"tx-bytes,omitempty"`
				TxDropped int `json:"tx-dropped,omitempty"`
				TxErrs    int `json:"tx-errs,omitempty"`
				TxPackets int `json:"tx-packets,omitempty"`
			} `json:"statistics,omitempty"`
			HardwareAddress string `json:"hardware-address,omitempty"`
		} `json:"return"`
	}

	var err error
	var s networkRaw

	rawJSON, err := qemuAgentCommand(ctx, d, "{\"execute\":\"guest-network-get-interfaces\"}")
	if err != nil {
		return []guestNetwork{}, err
	}

	err = json.Unmarshal([]byte(rawJSON), &s)
	if err != nil {
		fail.Printf("%sfailed to unmarshal Qemu-Agent JSON output: %s", id, err.Error())
		return []guestNetwork{}, err
	}

	networks := make([]guestNetwork, 0, 64)

	for _, net := range s.Return {

		var network guestNetwork

		network.Name = net.Name
		network.HardwareAddress = net.HardwareAddress
		network.Statistics.RxBytes = net.Statistics.RxBytes
		network.Statistics.RxDropped = net.Statistics.RxDropped
		network.Statistics.RxErrs = net.Statistics.RxErrs
		network.Statistics.RxPackets = net.Statistics.RxPackets
		network.Statistics.TxBytes = net.Statistics.TxBytes
		network.Statistics.TxDropped = net.Statistics.TxDropped
		network.Statistics.TxErrs = net.Statistics.TxErrs
		network.Statistics.TxPackets = net.Statistics.TxPackets

		ipAddrs := make([]guestNetworkIPAddress, 0, len(net.IPAddresses))

		for _, ip := range net.IPAddresses {

			var ipAddr guestNetworkIPAddress

			ipAddr.IPAddress = ip.IPAddress
			ipAddr.IPAddressType = ip.IPAddressType
			ipAddr.Prefix = ip.Prefix

			ipAddrs = append(ipAddrs, ipAddr)
		}

		network.IPAddresses = ipAddrs
		networks = append(networks, network)
	}

	return networks, nil
}

func getGuestPing(ctx context.Context, d *libvirt.Domain) bool {

	id := getReqIDFromContext(ctx)

	type pingRaw struct {
		Return struct {
		} `json:"return"`
	}

	var err error
	var s pingRaw

	rawJSON, err := qemuAgentCommand(ctx, d, "{\"execute\":\"guest-ping\"}")
	if err != nil {
		return false
	}

	err = json.Unmarshal([]byte(rawJSON), &s)
	if err != nil {
		fail.Printf("%sfailed to unmarshal Qemu-Agent JSON output: %s", id, err.Error())
		return false
	}

	return true
}

func getGuestAgentVersion(ctx context.Context, d *libvirt.Domain) (string, error) {

	id := getReqIDFromContext(ctx)

	type guestInfoRaw struct {
		Return struct {
			Version string `json:"version,omitempty"`
		} `json:"return"`
	}

	var err error
	var s guestInfoRaw

	// virsh qemu-agent-command DOMAIN '{"execute":"guest-info"}'
	rawJSON, err := qemuAgentCommand(ctx, d, "{\"execute\":\"guest-info\"}")
	if err != nil {
		return "", err
	}

	err = json.Unmarshal([]byte(rawJSON), &s)
	if err != nil {
		fail.Printf("%sfailed to unmarshal Qemu-Agent JSON output: %s", id, err.Error())
		return "", err
	}

	return s.Return.Version, nil
}

func qemuAgentCommand(ctx context.Context, d *libvirt.Domain, cmd string) (string, error) {

	id := getReqIDFromContext(ctx)

	out, err := d.QemuAgentCommand(cmd, libvirt.DOMAIN_QEMU_AGENT_COMMAND_DEFAULT, 0)
	if err != nil {
		fail.Printf("%sQEMU Agent command \"%s\" failed: %s\n", id, cmd, err.Error())
		return "", err
	}

	info.Printf("%sQEMU Agent command \"%s\" out: %s\n", id, cmd, out)
	return out, nil
}

func isGuestAgentAvailable(ctx context.Context, d *libvirt.Domain) bool {

	id := getReqIDFromContext(ctx)

	// _, _, err := d.GetTime(0)
	// if err != nil {
	// 	fail.Printf("%sQEMU Agent not available on domain: %s\n", id, err.Error())
	// 	return false
	// }

	ok := getGuestPing(ctx, d)
	if !ok {
		fail.Printf("%sQEMU Agent not available on domain\n", id)
		return false
	}

	info.Printf("%sQEMU Agent is available on domain\n", id)
	return true
}

func getGuestTime(ctx context.Context, d *libvirt.Domain) (int64, error) {

	id := getReqIDFromContext(ctx)

	time, _, err := d.GetTime(0)
	if err != nil {
		fail.Printf("%sfailed to get Guest time: %s\n", id, err.Error())
		return 0, err
	}

	info.Printf("%sGuest time is %d since Unix epoch\n", id, time)
	return time, nil
}

/*
func setGuestTime(ctx context.Context, d *libvirt.Domain, time int64) error {

	id := getReqIDFromContext(ctx)

	err := d.SetTime(time, 0, libvirt.DOMAIN_TIME_SYNC)
	if err != nil {
		fail.Printf("%sfailed to set Guest time: %s\n", id, err.Error())
		return err
	}

	info.Printf("%sGuest time is set to %d since Unix epoch\n", id, time)
	return nil
}
*/

/*
func getGuestOSType(ctx context.Context, d *libvirt.Domain) (string, error) {

	id := getReqIDFromContext(ctx)

	osType, err := d.GetOSType()
	if err != nil {
		fail.Printf("%sfailed to get Guest OS Type: %s\n", id, err.Error())
		return "", err
	}

	info.Printf("%sGuest OS Type is %s\n", id, osType)
	return osType, nil
}
*/

/*
func getGuestHostname(ctx context.Context, d *libvirt.Domain) (string, error) {

	id := getReqIDFromContext(ctx)

	host, err := d.GetHostname(0)
	if err != nil {
		fail.Printf("%sfailed to get Guest Hostname: %s\n", id, err.Error())
		return "", err
	}

	info.Printf("%sGuest Hostname is %s\n", id, host)
	return host, nil
}
*/

func getGuestFSInfo(ctx context.Context, d *libvirt.Domain) ([]libvirt.DomainFSInfo, error) {

	id := getReqIDFromContext(ctx)

	fs, err := d.GetFSInfo(0)
	if err != nil {
		fail.Printf("%sfailed to get Guest Filesystem info: %s\n", id, err.Error())
		return []libvirt.DomainFSInfo{}, err
	}

	info.Printf("%sacquired Guest filesystem info\n", id)
	return fs, nil
}

func setGuestPassword(ctx context.Context, d *libvirt.Domain, user string, password string) error {

	id := getReqIDFromContext(ctx)

	err := d.SetUserPassword(user, password, 0)
	if err != nil {
		fail.Printf("%sfailed to set Guest user password for: %s, %s\n", id, user, err.Error())
		return err
	}

	info.Printf("%sGuest user password is set for %s\n", id, user)
	return nil
}
