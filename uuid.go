package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"regexp"
	"strings"
)

// https://en.wikipedia.org/wiki/Universally_unique_identifier {Version 4 (random)}
func genUUID(ctx context.Context) string {

	id := getReqIDFromContext(ctx)

	u := make([]byte, 16)
	_, err := rand.Read(u)
	if err != nil {
		fail.Printf("%sfailed to generate pseudo-random UUID: %s\n", id, err.Error())
		// return "0d15ea5e-dead-dead-dead-defec8eddead"
		return ""
	}

	// this make sure that the 13th character is "4"
	u[6] = (u[6] | 0x40) & 0x4F
	// this make sure that the 17th is "8", "9", "a", or "b"
	u[8] = (u[8] | 0x80) & 0xBF

	uuid := fmt.Sprintf("%X-%X-%X-%X-%X", u[0:4], u[4:6], u[6:8], u[8:10], u[10:])
	uuid = strings.ToLower(uuid)

	info.Printf("%sgenerated pseudo-random UUID: %s\n", id, uuid)
	return uuid
}

func isUUIDValid(ctx context.Context, uuid string) (bool, error) {

	id := getReqIDFromContext(ctx)

	uuidPattern := "^([0-9]|[abcdef]|[ABCDEF]){8}-([0-9]|[abcdef]|[ABCDEF]){4}-([0-9]|[abcdef]|[ABCDEF]){4}-([0-9]|[abcdef]|[ABCDEF]){4}-([0-9]|[abcdef]|[ABCDEF]){12}$"

	ok, err := regexp.Match(uuidPattern, []byte(uuid))
	if err != nil {
		fail.Printf("%snot valid UUID %s: %s\n", id, uuid, err.Error())
		return false, fmt.Errorf("not valid UUID %s: %s", uuid, err.Error())
	}

	if !ok {
		fail.Printf("%snot valid UUID %s\n", id, uuid)
		return false, fmt.Errorf("not valid UUID %s", uuid)
	}

	info.Printf("%svalid UUID: %s\n", id, uuid)
	return true, nil
}
