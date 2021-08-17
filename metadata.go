package main

import (
	"context"
	"encoding/xml"
	"errors"
	"strconv"

	"github.com/libvirt/libvirt-go"
)

/* global variable declaration, if any... */
const metaTrust = "trust"
const metaQos = "qos"
const metaSpoofChk = "spoofchk"
const metaQueryRss = "query_rss"
const metaMaxTxRate = "max_tx_rate"

/*
func applyDomainMetadata(ctx context.Context, c *libvirt.Connect, dev netInfo) error {

	id := getReqIDFromContext(ctx)

	var cmd string
	var err error

	vf := strings.Replace(dev.VFName, "vf", "", -1)

	cmd = fmt.Sprintf("ip link set %s vf %s max_tx_rate %d", dev.PFName, vf, dev.Metadata.MaxTxRate)
	_, err = shellExec(ctx, cmd)
	if err != nil {
		fail.Printf("%sfailed to apply max_tx_rate setting to network interface: %s\n", id, err.Error())
		return err
	}

	cmd = fmt.Sprintf("ip link set %s vf %s trust %s", dev.PFName, vf, dev.Metadata.Trust)
	_, err = shellExec(ctx, cmd)
	if err != nil {
		fail.Printf("%sfailed to apply trust setting to network interface: %s\n", id, err.Error())
		return err
	}

	cmd = fmt.Sprintf("ip link set %s vf %s spoofchk %s", dev.PFName, vf, dev.Metadata.SpoofChk)
	_, err = shellExec(ctx, cmd)
	if err != nil {
		fail.Printf("%sfailed to apply spoofchk setting to network interface: %s\n", id, err.Error())
		return err
	}

	cmd = fmt.Sprintf("ip link set %s vf %s query_rss %s", dev.PFName, vf, dev.Metadata.QueryRss)
	_, err = shellExec(ctx, cmd)
	if err != nil {
		fail.Printf("%sfailed to apply query_rss setting to network interface: %s\n", id, err.Error())
		return err
	}

	cmd = fmt.Sprintf("ip link set %s vf %s vlan %s qos %d", dev.PFName, vf, dev.PVID, dev.Metadata.QoS)
	_, err = shellExec(ctx, cmd)
	if err != nil {
		fail.Printf("%sfailed to apply qos setting to network interface: %s\n", id, err.Error())
		return err
	}

	info.Printf("%sapplied additional network settings to hostdev network device(%s)\n", id, dev.MAC)
	return nil
}
*/

func getDomainMetadata(ctx context.Context, d *libvirt.Domain) (netMetadata, error) {

	id := getReqIDFromContext(ctx)

	type Network struct {
		Type  string `xml:"type,attr"`
		Value string `xml:",chardata"`
	}

	type Custom struct {
		XMLName xml.Name   `xml:"custom,omitempty"`
		Network []*Network `xml:"network,omitempty"`
	}

	var meta netMetadata
	var v Custom

	flags := libvirt.DOMAIN_AFFECT_CURRENT

	data, err := d.GetMetadata(libvirt.DOMAIN_METADATA_ELEMENT, "1c5537ac-8c84-4313-a8e7-9dd8d45ac7ed", flags)
	if err != nil {
		fail.Printf("%sfailed to get domain network metadata: %s\n", id, err.Error())
		return netMetadata{}, err
	}
	info.Printf("%sacquired domain metadata\n", id)

	err = xml.Unmarshal([]byte(data), &v)
	if err != nil {
		fail.Printf("%sfailed to unmarshal metadata XML: %s", id, err.Error())
		return netMetadata{}, err
	}

	if v.Network != nil {
		for _, el := range v.Network {
			switch el.Type {
			case metaMaxTxRate:
				meta.MaxTxRate, err = stringToUInteger(ctx, el.Value)
				if err != nil {
					return netMetadata{}, err
				}
			case metaTrust:
				meta.Trust = el.Value
			case metaSpoofChk:
				meta.SpoofChk = el.Value
			case metaQueryRss:
				meta.QueryRss = el.Value
			case metaQos:
				meta.QoS, err = stringToUInteger(ctx, el.Value)
				if err != nil {
					return netMetadata{}, err
				}
			}
		}
	} else {
		fail.Printf("%sfailed to acquire network metadata for domain: %s\n", id, errors.New("empty network section in XML"))
		return netMetadata{}, errors.New("empty network section in XML")
	}

	info.Printf("%sacquired network metadata for domain\n", id)
	return meta, nil
}

/*
EXAMPLE:
  virsh metadata --config --domain ubuntu-16.04 \
      --uri 1c5537ac-8c84-4313-a8e7-9dd8d45ac7ed \
      --key my \
      --set '
      <custom>
        <network type="max_tx_rate">100</network>
        <network type="trust">off</network>
        <network type="spoofchk">on</network>
        <network type="query_rss">off</network>
        <network type="qos">0</network>
    </custom>'
*/
func setDomainMetadataNetworkRate(ctx context.Context, d *libvirt.Domain, rate uint) (bool, error) {

	// Achtung: only for domain in shutdown state!

	id := getReqIDFromContext(ctx)

	isActive := isDomainActive(ctx, d)
	if isActive {
		return false, errors.New("domain must not be active while setting speed for network device")
	}

	type Network struct {
		Type  string `xml:"type,attr"`
		Value string `xml:",chardata"`
	}

	type Custom struct {
		XMLName xml.Name   `xml:"custom,omitempty"`
		Network []*Network `xml:"network,omitempty"`
	}

	var custom Custom
	var maxTxRate, trust, spoofChk, queryRss, qos Network

	meta, err := getDomainMetadata(ctx, d)
	if err != nil {
		return false, err
	}

	maxTxRate.Type = metaMaxTxRate
	maxTxRate.Value = strconv.Itoa(int(rate))

	qos.Type = metaQos
	qos.Value = strconv.Itoa(int(meta.QoS))

	trust.Type = metaTrust
	if len(meta.Trust) == 0 {
		trust.Value = off
	} else {
		trust.Value = meta.Trust
	}

	spoofChk.Type = metaSpoofChk
	if len(meta.SpoofChk) == 0 {
		spoofChk.Value = "on"
	} else {
		spoofChk.Value = meta.SpoofChk
	}

	queryRss.Type = metaQueryRss
	if len(meta.QueryRss) == 0 {
		queryRss.Value = "off"
	} else {
		queryRss.Value = meta.QueryRss
	}

	custom.Network = append(custom.Network, &maxTxRate)
	custom.Network = append(custom.Network, &qos)
	custom.Network = append(custom.Network, &trust)
	custom.Network = append(custom.Network, &spoofChk)
	custom.Network = append(custom.Network, &queryRss)

	bytes, err := xml.Marshal(custom)
	if err != nil {
		fail.Printf("%sfailed to marshal network metadata for domain: %s\n", id, err.Error())
		return false, nil
	}

	data := string(bytes)
	flags := libvirt.DOMAIN_AFFECT_CURRENT

	err = d.SetMetadata(libvirt.DOMAIN_METADATA_ELEMENT, data, "my", "1c5537ac-8c84-4313-a8e7-9dd8d45ac7ed", flags)
	if err != nil {
		fail.Printf("%sfailed to set network metadata for domain: %s\n", id, err.Error())
		return false, err
	}

	info.Printf("%supdated network metadata and hostdev speed for domain\n", id)
	return true, nil
}
