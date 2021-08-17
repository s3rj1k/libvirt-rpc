package main

import (
	"context"
	"strconv"
)

func stringToUInteger(ctx context.Context, s string) (uint, error) {

	id := getReqIDFromContext(ctx)

	i, err := strconv.Atoi(s)
	if err != nil {
		fail.Printf("%sfailed to convert string into integer\n", id)
		return 0, err
	}

	info.Printf("%sconverted string into integer\n", id)
	return uint(i), nil
}

/*
func getStringInBetween(ctx context.Context, str string, start string, end string) (result string) {

	s := strings.Index(str, start)
	if s == -1 {
		return ""
	}

	s += len(start)
	e := strings.Index(str, end)

	return str[s:e]
}
*/

/*
func shellExec(ctx context.Context, command string) (string, error) {

	id := getReqIDFromContext(ctx)

	out, err := exec.Command("/bin/sh", "-c", command).Output()
	output := strings.TrimSpace(string(out[:]))
	sanitized := strings.Replace(output, "\n", ";", -1)
	if err != nil {
		fail.Printf("%sfailed to exec: %s, output: %s, error: %s\n", id, command, sanitized, err.Error())
		return output, err
	}
	info.Printf("%sexecuted: %s, output: %s\n", id, command, sanitized)
	return output, nil
}
*/
