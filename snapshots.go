package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/libvirt/libvirt-go"
	"github.com/libvirt/libvirt-go-xml"
)

func getSnapshotName(ctx context.Context, s *libvirt.DomainSnapshot) (string, error) {

	id := getReqIDFromContext(ctx)

	name, err := s.GetName()
	if err != nil {
		fail.Printf("%sfailed to get snapshot name: %s", id, err.Error())
		return "", err
	}

	info.Printf("%sacquired snapshot name: %s\n", id, name)
	return name, nil
}

/*
func listAllDomainSnapshots(ctx context.Context, d *libvirt.Domain) ([]libvirt.DomainSnapshot, error) {

	id := getReqIDFromContext(ctx)

	snaps, err := d.ListAllSnapshots(libvirt.DOMAIN_SNAPSHOT_LIST_ROOTS)
	if err != nil {
		fail.Printf("%sfailed to find snapshots for domain: %s\n", id, err.Error())
		return []libvirt.DomainSnapshot{}, err
	}
	info.Printf("%sacquired list of snapshots for domain\n", id)

	return snaps, nil
}
*/

func getSnapshotParent(ctx context.Context, s *libvirt.DomainSnapshot) (*libvirt.DomainSnapshot, error) {

	id := getReqIDFromContext(ctx)

	parent, err := s.GetParent(0)
	if err != nil {
		fail.Printf("%sfailed to get snapshot parent: %s", id, err.Error())
		return nil, err
	}

	info.Printf("%sacquired snapshot parent\n", id)
	return parent, nil
}

func isSnapshotCurrent(ctx context.Context, s *libvirt.DomainSnapshot) (bool, error) {

	id := getReqIDFromContext(ctx)

	isCurrent, err := s.IsCurrent(0)
	if err != nil {
		fail.Printf("%sfailed to get snapshot current status: %s", id, err.Error())
		return false, err
	}

	info.Printf("%sacquired snapshot current status: %t\n", id, isCurrent)
	return isCurrent, nil
}

func freeSnapshot(ctx context.Context, s *libvirt.DomainSnapshot) error {

	id := getReqIDFromContext(ctx)

	err := s.Free()
	if err != nil {
		fail.Printf("%sfailed to free domain snapshot object: %s", id, err.Error())
		return err
	}

	info.Printf("%sfreed domain snapshot object\n", id)
	return nil
}

func countDomainSnapshotsWithFlags(ctx context.Context, d *libvirt.Domain, flags libvirt.DomainSnapshotListFlags) (int, error) {

	id := getReqIDFromContext(ctx)

	count, err := d.SnapshotNum(flags)
	if err != nil {
		fail.Printf("%sfailed to get snapshot count for domain: %s", id, err.Error())
		return 0, err
	}

	info.Printf("%sdomain snapshot count %d with flag %d\n", id, count, flags)
	return count, nil
}

func countDomainSnapshotChildrenWithFlags(ctx context.Context, s *libvirt.DomainSnapshot, flags libvirt.DomainSnapshotListFlags) (int, error) {

	id := getReqIDFromContext(ctx)

	count, err := s.NumChildren(flags)
	if err != nil {
		fail.Printf("%sfailed to get snapshot children count for domain: %s", id, err.Error())
		return 0, err
	}

	info.Printf("%sdomain snapshot children count %d with flag %d\n", id, count, flags)
	return count, nil
}

func isSnapshotHasFlags(ctx context.Context, d *libvirt.Domain, flags libvirt.DomainSnapshotListFlags, name string) (bool, error) {

	id := getReqIDFromContext(ctx)

	snapNames, err := d.SnapshotListNames(flags)
	if err != nil {
		fail.Printf("%sfailed to get snapshot list for domain: %s", id, err.Error())
		return false, err
	}
	info.Printf("%sacquired domain snapshot list\n", id)

	sanitizedName := strings.ToLower(strings.TrimSpace(name))

	for _, v := range snapNames {
		sanitizedValue := strings.ToLower(strings.TrimSpace(v))
		if sanitizedValue == sanitizedName {
			info.Printf("%sdomain snapshot has flag: %d\n", id, flags)
			return true, nil
		}
	}

	info.Printf("%sdomain snapshot has no flag: %d\n", id, flags)
	return false, nil
}

func listDomainSnapshots(ctx context.Context, d *libvirt.Domain) []snapshotInfo {

	id := getReqIDFromContext(ctx)

	// flags := libvirt.DOMAIN_SNAPSHOT_LIST_INTERNAL | libvirt.DOMAIN_SNAPSHOT_LIST_INACTIVE
	flags := libvirt.DomainSnapshotListFlags(0)

	snaps, err := d.ListAllSnapshots(flags)
	if err != nil {
		return []snapshotInfo{}
	}

	snapshotsInfo := make([]snapshotInfo, 0, len(snaps))

	for _, snap := range snaps {

		var err error
		var isCurrent, isFlag bool
		var name, parentName string
		var parent *libvirt.DomainSnapshot
		var snapshotInfo snapshotInfo

		name, err = getSnapshotName(ctx, &snap)
		if err != nil {
			continue
		}

		snapshotInfo.Name = name
		info.Printf("%sfound domain snapshot %s\n", id, name)

		snapshotInfo.ChildrenCount, err = countDomainSnapshotChildrenWithFlags(ctx, &snap, libvirt.DomainSnapshotListFlags(0))
		if err != nil {
			snapshotInfo.ChildrenCount = 0
		}

		isFlag, err = isSnapshotHasFlags(ctx, d, libvirt.DOMAIN_SNAPSHOT_LIST_ROOTS, name)
		if err != nil {
			snapshotInfo.Error = true
			snapshotInfo.ErrorMessage = append(snapshotInfo.ErrorMessage, err.Error())
		}
		snapshotInfo.HasNoParents = isFlag

		if !snapshotInfo.HasNoParents {

			parent, err = getSnapshotParent(ctx, &snap)
			if err != nil {
				snapshotInfo.Error = true
				snapshotInfo.ErrorMessage = append(snapshotInfo.ErrorMessage, err.Error())
			}

			if parent != nil {
				info.Printf("%sfound domain snapshot %s parent\n", id, name)

				parentName, err = getSnapshotName(ctx, parent)
				if err != nil {
					snapshotInfo.Error = true
					snapshotInfo.ErrorMessage = append(snapshotInfo.ErrorMessage, err.Error())
				}

				snapshotInfo.Parent = parentName
				info.Printf("%sfound domain snapshot %s parent name %s\n", id, name, parentName)
			}
		} else {
			snapshotInfo.Parent = "/"
		}

		isCurrent, err = isSnapshotCurrent(ctx, &snap)
		if err != nil {
			snapshotInfo.Error = true
			snapshotInfo.ErrorMessage = append(snapshotInfo.ErrorMessage, err.Error())
		}
		snapshotInfo.IsCurrent = isCurrent

		err = freeSnapshot(ctx, &snap)
		if err != nil {
			snapshotInfo.Error = true
			snapshotInfo.ErrorMessage = append(snapshotInfo.ErrorMessage, err.Error())
		}

		if parent != nil {
			err := freeSnapshot(ctx, parent)
			if err != nil {
				snapshotInfo.Error = true
				snapshotInfo.ErrorMessage = append(snapshotInfo.ErrorMessage, err.Error())
			}
		}

		isFlag, err = isSnapshotHasFlags(ctx, d, libvirt.DOMAIN_SNAPSHOT_LIST_INTERNAL, name)
		if err != nil {
			snapshotInfo.Error = true
			snapshotInfo.ErrorMessage = append(snapshotInfo.ErrorMessage, err.Error())
		}
		snapshotInfo.IsInternal = isFlag

		isFlag, err = isSnapshotHasFlags(ctx, d, libvirt.DOMAIN_SNAPSHOT_LIST_EXTERNAL, name)
		if err != nil {
			snapshotInfo.Error = true
			snapshotInfo.ErrorMessage = append(snapshotInfo.ErrorMessage, err.Error())
		}
		snapshotInfo.IsExternal = isFlag

		isFlag, err = isSnapshotHasFlags(ctx, d, libvirt.DOMAIN_SNAPSHOT_LIST_DISK_ONLY, name)
		if err != nil {
			snapshotInfo.Error = true
			snapshotInfo.ErrorMessage = append(snapshotInfo.ErrorMessage, err.Error())
		}
		snapshotInfo.IsDiskOnly = isFlag

		isFlag, err = isSnapshotHasFlags(ctx, d, libvirt.DOMAIN_SNAPSHOT_LIST_ACTIVE, name)
		if err != nil {
			snapshotInfo.Error = true
			snapshotInfo.ErrorMessage = append(snapshotInfo.ErrorMessage, err.Error())
		}
		snapshotInfo.WasActive = isFlag

		isFlag, err = isSnapshotHasFlags(ctx, d, libvirt.DOMAIN_SNAPSHOT_LIST_INACTIVE, name)
		if err != nil {
			snapshotInfo.Error = true
			snapshotInfo.ErrorMessage = append(snapshotInfo.ErrorMessage, err.Error())
		}
		snapshotInfo.WasInactive = isFlag

		isFlag, err = isSnapshotHasFlags(ctx, d, libvirt.DOMAIN_SNAPSHOT_LIST_METADATA, name)
		if err != nil {
			snapshotInfo.Error = true
			snapshotInfo.ErrorMessage = append(snapshotInfo.ErrorMessage, err.Error())
		}
		snapshotInfo.HasMetadata = isFlag

		isFlag, err = isSnapshotHasFlags(ctx, d, libvirt.DOMAIN_SNAPSHOT_LIST_NO_METADATA, name)
		if err != nil {
			snapshotInfo.Error = true
			snapshotInfo.ErrorMessage = append(snapshotInfo.ErrorMessage, err.Error())
		}
		snapshotInfo.HasNoMetadata = isFlag

		isFlag, err = isSnapshotHasFlags(ctx, d, libvirt.DOMAIN_SNAPSHOT_LIST_LEAVES, name)
		if err != nil {
			snapshotInfo.Error = true
			snapshotInfo.ErrorMessage = append(snapshotInfo.ErrorMessage, err.Error())
		}
		snapshotInfo.HasNoChildren = isFlag

		isFlag, err = isSnapshotHasFlags(ctx, d, libvirt.DOMAIN_SNAPSHOT_LIST_NO_LEAVES, name)
		if err != nil {
			snapshotInfo.Error = true
			snapshotInfo.ErrorMessage = append(snapshotInfo.ErrorMessage, err.Error())
		}
		snapshotInfo.HasChildren = isFlag

		snapshotsInfo = append(snapshotsInfo, snapshotInfo)
	}

	return snapshotsInfo
}

func prepareXMLForSnapshot(ctx context.Context, d *libvirt.Domain, name string, isInternal bool) (string, error) {

	id := getReqIDFromContext(ctx)

	domain := getDomainName(ctx, d)

	xmlDoc, err := d.GetXMLDesc(libvirt.DOMAIN_XML_INACTIVE)
	if err != nil {
		fail.Printf("%sfailed to get domain XML: %s\n", id, err.Error())
		return "", err
	}
	info.Printf("%sacquired domain XML\n", id)

	domCfg := &libvirtxml.Domain{}
	err = domCfg.Unmarshal(xmlDoc)
	if err != nil {
		fail.Printf("%sfailed to parse domain XML: %s\n", id, err.Error())
		return "", err
	}
	info.Printf("%sparsed domain XML\n", id)

	var snapshotType string
	if isInternal {
		snapshotType = "internal"
	} else {
		snapshotType = "external"
	}

	disksXML := make([]string, 0)
	if domCfg != nil {
		if domCfg.Devices != nil {
			for _, disk := range domCfg.Devices.Disks {
				if disk.Device == "disk" && disk.ReadOnly == nil && disk.Shareable == nil && disk.Transient == nil {
					if disk.Target != nil {
						disksXML = append(disksXML, fmt.Sprintf("<disk name='%s' snapshot='%s'/>", disk.Target.Dev, snapshotType))
						info.Printf("%sfound disk %s\n", id, disk.Target.Dev)
					}
				}
			}
		} else {
			fail.Printf("%sfailed to parse domain XML: %s\n", id, errors.New("domain xml device section is empty"))
			return "", err
		}
	} else {
		fail.Printf("%sfailed to parse domain XML: %s\n", id, errors.New("domain xml is empty"))
		return "", err
	}

	if len(disksXML) == 0 {
		fail.Printf("%sfailed to parse domain XML: %s\n", id, errors.New("no disk found"))
		return "", err
	}

	// https://libvirt.org/formatsnapshot.html
	description := fmt.Sprintf("snapshot named as: %s; for: %s; taken at: %s", name, domain, time.Now().Format(time.RFC3339))

	xml := fmt.Sprintf(`<domainsnapshot>
                        <name>%s</name>
                        <description>%s</description>
                        <disks>%s</disks>
                      </domainsnapshot>`, name, description, strings.Join(disksXML, ""))

	info.Printf("%sprepared snapshot XML\n", id)
	return xml, nil
}

func makeDomainSnapshot(ctx context.Context, d *libvirt.Domain, flags libvirt.DomainSnapshotCreateFlags, xml string) (bool, error) {

	id := getReqIDFromContext(ctx)

	snap, err := d.CreateSnapshotXML(xml, flags)
	if err != nil || snap == nil {
		fail.Printf("%sfailed to take domain snapshot: %s\n", id, err.Error())
		return false, err
	}

	info.Printf("%screated domain snapshot\n", id)

	defer func() {

		err = freeSnapshot(ctx, snap)

		if err != nil {
			fail.Printf("%sfailed in defer: %s", id, err.Error())
		}

	}()

	return true, nil
}

func lookupDomainSnapshotByName(ctx context.Context, d *libvirt.Domain, name string) (*libvirt.DomainSnapshot, error) {

	id := getReqIDFromContext(ctx)

	snap, err := d.SnapshotLookupByName(name, 0)
	if err != nil || snap == nil {
		fail.Printf("%sfailed to find domain snapshot: %s\n", id, err.Error())
		return nil, err
	}

	info.Printf("%sfound domain snapshot %s\n", id, name)
	return snap, nil
}

func deleteSnapshot(ctx context.Context, s *libvirt.DomainSnapshot, flags libvirt.DomainSnapshotDeleteFlags) (bool, error) {

	id := getReqIDFromContext(ctx)

	err := s.Delete(flags)
	if err != nil {
		fail.Printf("%sfailed to delete domain snapshot: %s\n", id, err.Error())
		return false, err
	}

	info.Printf("%sdeleted domain snapshot\n", id)
	return true, nil
}

func revertToSnapshot(ctx context.Context, s *libvirt.DomainSnapshot) (bool, error) {

	id := getReqIDFromContext(ctx)

	err := s.RevertToSnapshot(0)
	if err != nil {
		fail.Printf("%sfailed to revert to domain snapshot: %s\n", id, err.Error())
		return false, err
	}

	info.Printf("%sreverted to domain snapshot\n", id)

	defer func() {

		err = freeSnapshot(ctx, s)

		if err != nil {
			fail.Printf("%sfailed in defer: %s", id, err.Error())
		}

	}()

	return true, nil
}

func blockCommitActive(ctx context.Context, d *libvirt.Domain, disk string) (bool, error) {

	id := getReqIDFromContext(ctx)

	err := d.BlockCommit(disk, "", "", 0, libvirt.DOMAIN_BLOCK_COMMIT_ACTIVE|libvirt.DOMAIN_BLOCK_COMMIT_SHALLOW)
	if err != nil {
		fail.Printf("%sfailed to do active block commit %s: %s\n", id, disk, err.Error())
		return false, err
	}

	info.Printf("%sstarted active block commit operation for %s\n", id, disk)
	return true, nil
}

func blockCommitActivePivot(ctx context.Context, d *libvirt.Domain, disk string) (bool, error) {

	id := getReqIDFromContext(ctx)

	err := d.BlockJobAbort(disk, libvirt.DOMAIN_BLOCK_JOB_ABORT_PIVOT|libvirt.DOMAIN_BLOCK_JOB_ABORT_ASYNC)
	if err != nil {
		fail.Printf("%sfailed to pivot active block commit %s: %s\n", id, disk, err.Error())
		return false, err
	}

	info.Printf("%spivot for active block commit on disk %s completed\n", id, disk)
	return true, nil
}

func isDomainBlockJobRunning(ctx context.Context, d *libvirt.Domain) (bool, error) {

	id := getReqIDFromContext(ctx)

	isActive := isDomainActive(ctx, d)
	if !isActive {
		info.Printf("%sdomain has no active block jobs\n", id)
		return false, nil
	}

	disks, err := getDomainBlockDeviceNamesOrPaths(ctx, d, false)
	if err != nil {
		return false, err
	}

	for _, disk := range disks {
		jobInfo, err := getDomainBlockJobInfo(ctx, d, disk)
		if err != nil {
			continue
		}

		switch jobInfo.Type {
		case domainBlockJobTypePull:
			info.Printf("%sdomain has active block job, backup in progress\n", id)
			return true, nil
		case domainBlockJobTypeCopy:
			info.Printf("%sdomain has active block job, backup in progress\n", id)
			return true, nil
		case domainBlockJobTypeCommit:
			info.Printf("%sdomain has active block job, backup in progress\n", id)
			return true, nil
		case domainBlockJobTypeActiveCommit:
			info.Printf("%sdomain has active block job, backup in progress\n", id)
			return true, nil
		}
	}

	info.Printf("%sdomain has no active block jobs\n", id)
	return false, nil
}

func isDomainBlockHasActiveExternalBackupSnashot(ctx context.Context, d *libvirt.Domain) (bool, error) {

	id := getReqIDFromContext(ctx)

	paths, err := getDomainBlockDeviceNamesOrPaths(ctx, d, true)
	if err != nil {
		return false, err
	}

	for _, path := range paths {
		if strings.HasSuffix(path, "external.snapshot.qcow2") {
			info.Printf("%sdomain has active external snapshot, backup gone wrong\n", id)
			return true, nil
		}
	}

	snapCount, err := countDomainSnapshotsWithFlags(ctx, d, libvirt.DOMAIN_SNAPSHOT_LIST_EXTERNAL)
	if err != nil {
		return false, err
	}
	if snapCount != 0 {
		info.Printf("%sdomain has active external snapshot\n", id)
		return true, nil
	}

	info.Printf("%sno active external snapshots, leftovers from broken backup found\n", id)
	return false, nil
}

func waitBlockCommitActive(ctx context.Context, d *libvirt.Domain, disk string) bool {

	id := getReqIDFromContext(ctx)

	t := time.Now()
	var j, retries uint
	retries = 3

	for {

		if time.Since(t).Seconds() > 60*60*1 {
			info.Printf("%sstopped waiting for active block job, timeout exceeded\n", id)
			return false
		}

		if j == retries {
			info.Printf("%sstopped waiting for active block job\n", id)
			break
		}

		time.Sleep(5 * time.Second)

		jobInfo, err := getDomainBlockJobInfo(ctx, d, disk)
		if err != nil {
			continue
		}

		if jobInfo.Cur == jobInfo.End && jobInfo.End == 0 {
			fail.Printf("%sactive block job for %s stopped unexpectedly\n", id, disk)
			break
		}

		if jobInfo.Type == domainBlockJobTypeActiveCommit && jobInfo.Cur == jobInfo.End && jobInfo.End > 0 {
			j = j + 1
			info.Printf("%sdomain has active block job, backup in progress: %d/%d, retries: %d/%d\n", id, jobInfo.Cur, jobInfo.End, j, retries)
		} else {
			info.Printf("%sdomain has active block job, backup in progress: %d/%d\n", id, jobInfo.Cur, jobInfo.End)
		}
	}

	return true
}
