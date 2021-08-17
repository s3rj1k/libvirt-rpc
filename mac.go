package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
)

func genMAC(ctx context.Context) string {

	id := getReqIDFromContext(ctx)

	b := make([]byte, 6)
	_, err := rand.Read(b)
	if err != nil {
		fail.Printf("%sfailed to generate pseudo-random QEMU-KVM MAC: %s\n", id, err.Error())
		return ""
	}

	mac := fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x", 0x52, 0x54, 0x00, b[0:1], b[2:3], b[4:5])
	mac = strings.ToLower(mac)

	info.Printf("%sgenerated pseudo-random QEMU-KVM MAC: %s\n", id, mac)
	return mac
}

func isMACvalid(ctx context.Context, mac string) (bool, error) {

	id := getReqIDFromContext(ctx)

	if !strings.HasPrefix(mac, "52:54:00") {
		fail.Printf("%sMAC: %s has non valid QEMU-KVM vendor prefix\n", id, mac)
		return false, fmt.Errorf("MAC: %s has non valid QEMU-KVM vendor prefix", mac)
	}

	macPattern := "^([0-9]|[abcdef]|[ABCDEF]){2}:([0-9]|[abcdef]|[ABCDEF]){2}:([0-9]|[abcdef]|[ABCDEF]){2}:([0-9]|[abcdef]|[ABCDEF]){2}:([0-9]|[abcdef]|[ABCDEF]){2}:([0-9]|[abcdef]|[ABCDEF]){2}$"
	ok, err := regexp.Match(macPattern, []byte(mac))
	if err != nil || !ok {
		fail.Printf("%snot valid MAC: %s\n", id, mac)
		return false, fmt.Errorf("not valid MAC: %s", mac)
	}

	info.Printf("%sQEMU-KVM MAC is valid: %s\n", id, mac)
	return true, nil
}
