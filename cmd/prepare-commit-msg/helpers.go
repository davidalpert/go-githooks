package main

import (
	"bytes"
	"fmt"
	"github.com/apex/log"
	"os"
	"os/exec"
	"strings"
)

func checkError(msg string, err error) {
	if err == nil {
		return
	}

	log.WithError(err).Error(msg)
	fmt.Printf("%s: %#v\n", msg, err)
	os.Exit(1)
}

/*
	cmd := exec.Command("tr", "a-z", "A-Z")
	cmd.Stdin = strings.NewReader("some input")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
 */
func execAndCaptureOutput(cmdDescription string, cmdName string, arg ...string) (string, error) {
	cmd := exec.Command(cmdName, arg...)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%s failed: %v", cmdDescription, err)
	}

	return strings.TrimSpace(out.String()), nil
}

func stringInSlice(s []string, v string) bool {
	for _, a := range s {
		if a == v {
			return true
		}
	}
	return false
}
