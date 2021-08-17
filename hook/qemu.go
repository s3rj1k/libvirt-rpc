package main

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/libvirt/libvirt-go-xml"
)

/*
Docs: https://www.libvirt.org/hooks.html
      https://libvirt.org/formatdomain.html#elementsMetadata

Hint: If you make a change you need to 'systemctl restart libvirtd'.

Debug: virsh dumpxml ubuntu-16.04 | ./qemu ubuntu-16.04 start begin -

XML:

<domain>
...
  <metadata>
    <my:custom xmlns:my="1c5537ac-8c84-4313-a8e7-9dd8d45ac7ed">
      <my:network type="max_tx_rate">100</my:network>
      <my:network type="trust">off</my:network>
      <my:network type="spoofchk">on</my:network>
      <my:network type="query_rss">off</my:network>
      <my:network type="qos">0</my:network>
    </my:custom>
  </metadata>
...
</domain>
*/

type netMetadata struct {
	MaxTxRate uint   `json:"MaxTxRate"` // Mbps
	QoS       uint   `json:"QoS"`
	Trust     string `json:"Trust"`
	SpoofChk  string `json:"SpoofChk"`
	QueryRss  string `json:"QueryRss"`
}

func stringToUInteger(s string) (uint, error) {

	i, err := strconv.Atoi(s)
	if err != nil {
		log.Println("failed to convert string into integer")
		return 0, err
	}

	return uint(i), nil
}

func uIntegerToString(i uint) string {
	return strconv.Itoa(int(i))
}

func parseMetadataXML(domCfg *libvirtxml.Domain) (netMetadata, error) {

	type Network struct {
		Type  string `xml:"type,attr"`
		Value string `xml:",chardata"`
	}

	type Custom struct {
		XMLName xml.Name   `xml:"custom,omitempty"`
		Network []*Network `xml:"network,omitempty"`
	}

	var out netMetadata
	var meta Custom
	var err error

	if domCfg.Metadata == nil {
		log.Println("metadata section not found in Domain XML")
		return netMetadata{}, errors.New("metadata section not found in Domain XML")
	}

	err = xml.Unmarshal([]byte(domCfg.Metadata.XML), &meta)
	if err != nil {
		log.Printf("failed to unmarshal metadata XML: %s", err.Error())
		return netMetadata{}, err
	}

	if meta.Network != nil {
		for _, el := range meta.Network {
			switch el.Type {
			case "max_tx_rate":
				out.MaxTxRate, err = stringToUInteger(el.Value)
				if err != nil {
					log.Printf("failed to unmarshal metadata XML: %s", err.Error())
					out.MaxTxRate = 100
				}
			case "trust":
				if el.Value == "off" || el.Value == "on" {
					out.Trust = el.Value
				} else {
					out.Trust = "off"
				}
			case "spoofchk":
				if el.Value == "off" || el.Value == "on" {
					out.SpoofChk = el.Value
				} else {
					out.SpoofChk = "on"
				}
			case "query_rss":
				if el.Value == "off" || el.Value == "on" {
					out.QueryRss = el.Value
				} else {
					out.QueryRss = "off"
				}
			case "qos":
				out.QoS, err = stringToUInteger(el.Value)
				if err != nil {
					log.Printf("failed to unmarshal metadata XML: %s", err.Error())
					out.QoS = 0
				}
			}
		}
	} else {
		log.Printf("failed to acquire network metadata: %s\n", errors.New("empty network section in XML"))
		return netMetadata{}, errors.New("empty network section in XML")
	}

	return out, nil
}

func validateInterfaceXML(net *libvirtxml.DomainInterface) bool {

	if net.Source == nil {
		log.Println("not valid network interface, skipping...")
		return false
	}

	if net.Source.Hostdev == nil {
		log.Println("not valid network interface, skipping...")
		return false
	}

	if net.Source.Hostdev.PCI == nil {
		log.Println("not valid network interface, skipping...")
		return false
	}

	if net.Source.Hostdev.PCI.Address == nil {
		log.Println("not valid network interface, skipping...")
		return false
	}

	if net.Alias == nil {
		log.Println("not valid network interface, skipping...")
		return false
	}

	if !strings.HasPrefix(net.Alias.Name, "hostdev") {
		log.Println("not valid network interface, skipping...")
		return false
	}

	if net.VLan == nil {
		log.Println("not valid network interface, skipping...")
		return false
	}

	if net.VLan.Trunk != "" {
		log.Println("not valid network interface, skipping...")
		return false
	}

	if len(net.VLan.Tags) != 1 {
		log.Println("not valid network interface, skipping...")
		return false
	}

	if net.VLan.Tags[0].NativeMode != "" {
		log.Println("not valid network interface, skipping...")
		return false
	}

	if net.XMLName.Local != "interface" {
		log.Println("not valid network interface, skipping...")
		return false
	}

	if net.Managed != "yes" {
		log.Println("not valid network interface, skipping...")
		return false
	}

	if net.MAC == nil {
		log.Println("not valid network interface, skipping...")
		return false
	}

	if net.MAC.Address == "00:00:00:00:00:00" {
		log.Println("not valid network interface, skipping...")
		return false
	}

	if !strings.HasPrefix(net.MAC.Address, "52:54:00:") {
		log.Println("not valid network interface, skipping...")
		return false
	}

	return true
}

func parseDomainXML(stdin io.Reader) (*libvirtxml.Domain, error) {

	scanner := bufio.NewScanner(stdin)
	lines := make([]string, 0)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lines = append(lines, line)
	}

	err := scanner.Err()
	if err != nil {
		log.Printf("failed to read domain xml: %s\n", err.Error())
		return &libvirtxml.Domain{}, err
	}

	xml := strings.Join(lines, "")

	domCfg := &libvirtxml.Domain{}

	err = domCfg.Unmarshal(xml)
	if err != nil {
		log.Printf("failed to unmarshal domain xml: %s\n", err.Error())
		return &libvirtxml.Domain{}, err
	}

	if domCfg.Devices == nil {
		log.Println("device section not found in Domain XML")
		return &libvirtxml.Domain{}, errors.New("device section not found in Domain XML")
	}

	return domCfg, nil
}

func parseIpRouteOutput(mac string) (string, string, error) {

	out, err := exec.Command("/sbin/ip", "-o", "link").Output()
	if err != nil {
		log.Printf("failed to get list of interface(s) link state(s): %s\n", err.Error())
		return "", "", err
	}

	rMAC := fmt.Sprintf("vf\\s(\\d+)\\sMAC\\s%s,", mac)
	for _, b := range bytes.Split(out, []byte("\n")) {

		ok, err := regexp.Match(rMAC, b)
		if err != nil {
			log.Printf("failed to parse \"ip link\" output: %s\n", err.Error())
			return "", "", err
		}

		if ok {
			rPF := regexp.MustCompile("^\\d+:\\s(\\w.+):\\s")
			rVF := regexp.MustCompile(rMAC)
			pf := rPF.FindSubmatch(b)[:]
			vf := rVF.FindSubmatch(b)[:]

			if pf == nil || len(pf) < 2 {
				log.Printf("failed to find PF interface name.")
				return "", "", errors.New("no interface found")
			}

			if vf == nil || len(vf) < 2 {
				log.Printf("failed to find VF interface number.")
				return "", "", errors.New("no interface found")
			}

			return string(pf[1]), string(vf[1]), nil
		}

	}

	log.Printf("failed to parse \"ip link\" output: %s\n", errors.New("no interface found"))
	return "", "", errors.New("no interface found")
}

func main() {

	if len(os.Args) != 5 {
		log.Println("incorrect number of arguments provided.")
		os.Exit(0)
	}

	ifaces := make([]map[string]string, 0, 64)

	// ./qemu ubuntu-16.04 {start} begin -
	switch os.Args[2] {
	case "start":

		// ./qemu ubuntu-16.04 start {begin} -
		switch os.Args[3] {
		case "begin":

			domCfg, err := parseDomainXML(os.Stdin)
			if err != nil {
				os.Exit(0)
			}

			if domCfg.Name != strings.TrimSpace(strings.ToLower(os.Args[1])) {
				log.Println("domain in arguments does not match that of in XML")
				os.Exit(0)
			}

			meta, err := parseMetadataXML(domCfg)
			if err != nil {
				os.Exit(0)
			}

			for _, net := range domCfg.Devices.Interfaces {

				ok := validateInterfaceXML(&net)
				if !ok {
					continue
				}

				var iface = map[string]string{
					"mac":         "00:00:00:00:00:00",
					"max_tx_rate": "100",
					"trust":       "off",
					"spoofchk":    "on",
					"query_rss":   "off",
					"qos":         "0",
					"pf":          "",
					"vf":          "",
					"vlan":        "",
				}

				iface["pf"], iface["vf"], err = parseIpRouteOutput(net.MAC.Address)
				if err != nil {
					continue
				}

				if iface["pf"] == "" || iface["vf"] == "" {
					continue
				}

				iface["mac"] = net.MAC.Address
				iface["vlan"] = uIntegerToString(net.VLan.Tags[0].ID)
				iface["max_tx_rate"] = uIntegerToString(meta.MaxTxRate)
				iface["qos"] = uIntegerToString(meta.QoS)
				iface["trust"] = meta.Trust
				iface["spoofchk"] = meta.SpoofChk
				iface["query_rss"] = meta.QueryRss

				ifaces = append(ifaces, iface)
			}

			for _, iface := range ifaces {

				cmds := make([]string, 0)

				for k, v := range iface {
					switch k {
					case "max_tx_rate":
						cmds = append(cmds, fmt.Sprintf("link set %s vf %s max_tx_rate %s", iface["pf"], iface["vf"], v))
					case "trust":
						cmds = append(cmds, fmt.Sprintf("link set %s vf %s trust %s", iface["pf"], iface["vf"], v))
					case "spoofchk":
						cmds = append(cmds, fmt.Sprintf("link set %s vf %s spoofchk %s", iface["pf"], iface["vf"], v))
					case "query_rss":
						cmds = append(cmds, fmt.Sprintf("link set %s vf %s query_rss %s", iface["pf"], iface["vf"], v))
					case "qos":
						cmds = append(cmds, fmt.Sprintf("link set %s vf %s vlan %s qos %s", iface["pf"], iface["vf"], iface["vlan"], v))
					}
				}

				// Batch applying iproute2 commands
				for _, cmd := range cmds {
					// variadic input
					out, err := exec.Command("ip", strings.Split(cmd, " ")...).CombinedOutput()
					if err != nil {
						log.Printf("Failed to run iproute2 command: %s %s\n", out, err.Error())
					}
				}
			}

		// END {begin}
		default:
			os.Exit(0)
		}

	// END {start}
	default:
		os.Exit(0)
	}

	// END main{}
	os.Exit(0)
}
