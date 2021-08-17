package main

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/libvirt/libvirt-go"
	"github.com/pierrec/lz4"
)

func createBackup(ctx context.Context, c *libvirt.Connect, inputFile string) error {

	id := getReqIDFromContext(ctx)

	t := time.Now()

	info.Printf("%sstarted backup for %s\n", id, inputFile)

	outputFile := fmt.Sprintf("%s_%s_backup%s", path.Clean(inputFile), t.Format("20060102150405"), lz4.Extension)
	_, err := lz4Compress(ctx, inputFile, outputFile)
	if err != nil {
		fail.Printf("%sfailed to create backup for %s: %s\n", id, inputFile, err.Error())
		return err
	}

	info.Printf("%sfinished backup for %s\n", id, inputFile)
	return nil
}

func deleteTemporaryExternalSnapshot(ctx context.Context, c *libvirt.Connect, paths []string) error {

	id := getReqIDFromContext(ctx)

	err := refreshAllStorgePools(ctx, c)
	if err != nil {
		return err
	}

	for _, path := range paths {

		if strings.HasPrefix(path, "/var/lib/libvirt/") &&
			strings.HasSuffix(path, ".external.snapshot.qcow2") &&
			!strings.Contains(path, " ") &&
			!strings.Contains(path, "../") &&
			!strings.Contains(path, "*") {

			vol, err := lookupStorageVolByPath(ctx, c, path)
			if err != nil {
				return err
			}

			err = deletePoolVolume(ctx, vol, libvirt.STORAGE_VOL_DELETE_NORMAL)
			if err != nil {
				fail.Printf("%sfailed to remove redundant external snapshot %s: %s\n", id, path, err.Error())
				return err
			}

			info.Printf("%sremoved redundant external snapshot %s\n", id, path)
		} else {

			info.Printf("%ssafety check, not removing: %s\n", id, path)
		}
	}

	return nil
}
