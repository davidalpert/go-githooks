package main

import (
	"fmt"
	"github.com/go-git/go-git/v5/config"
	config2 "github.com/go-git/go-git/v5/plumbing/format/config"
	"strconv"
	"strings"
)

func getRepoConfigOptionOrDefaultString(c *config.Config, section, subsection, key, defaultValue string) string {
	//fmt.Printf("reading %s | %s | %s (default: %s)\n", section, subsection, key, defaultValue)
	if !c.Raw.HasSection(section) {
		//fmt.Printf("couldn't find section '%s'\n", section)
		return defaultValue
	}

	s := c.Raw.Section(section)
	var o config2.Options
	if subsection == "" {
		//fmt.Printf("no sub-section given\n")
		o = s.Options
	} else if s.HasSubsection(subsection) {
		//fmt.Printf("has sub-section '%s'\n", subsection)
		o = s.Subsection(subsection).Options
	} else {
		//fmt.Printf("no sub-section '%s'\n", subsection)
		return defaultValue
	}

	if o.Has(key) {
		//fmt.Printf("has key '%s'\n", key)
		return o.Get(key)
	}
	//fmt.Printf("missing key '%s'\n", key)
	return defaultValue
}

func getRepoConfigOptionOrDefaultBool(c *config.Config, section, subsection, key string, defaultValue bool) bool {
	v := getRepoConfigOptionOrDefaultString(c, section, subsection, key, "")
	//fmt.Printf("(%s, %s, %s) got: %s\n", section, subsection, key, v)
	if v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			panic(fmt.Errorf("failed parsing '%s' as a bool: %v", v, err))
		}
		return b
	}
	return defaultValue
}

func getRepoConfigOptionOrDefaultSlice(c *config.Config, section, subsection, key string, defaultValues []string) []string {
	v := getRepoConfigOptionOrDefaultString(c, section, subsection, key, "")
	if v != "" {
		return strings.Split(v, ",")
	}
	return defaultValues
}

