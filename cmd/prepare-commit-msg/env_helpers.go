package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func getEnvOrDefaultString(envKey string, defaultValue string) string {
	v := os.Getenv(envKey)
	if v != "" {
		return v
	}
	return defaultValue
}

func getEnvOrDefaultBool(envKey string, defaultValue bool) bool {
	v := os.Getenv(envKey)
	if v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			panic(fmt.Errorf("failed parsing '%s' as a bool: %v", v, err))
		}
		return b
	}
	return defaultValue
}

func getEnvOrDefaultStringSlice(envKey string, defaults ...string) []string {
	v := os.Getenv(envKey)
	if v != "" {
		return strings.Split(v, ",")
	}
	return defaults
}

