// Copyright (C) 2023 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

/*
Package rdtcat implements functions to read RDT CAT configurations.

Classes configured in /sys/fs/resctrl are read and size as well as type of the
utilized cache are extracted.
*/
package rdtcat

import (
	"bufio"
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/intel/goresctrl/pkg/rdt"
	"github.com/rs/zerolog/log"
)

const (
	RdtctrlPath  = "/sys/fs/resctrl" // pseudo filesystem used to configure RDT CAT
	DefaultClass = "system/default"  // used for all PIDs if not assigned to specific class
)

// Resctrl keeps all info collected from the configured classes in /sys/fs/resctl.
type Resctrl struct {
	ExcatBuffers
	ResctrlGroups []ResctrlGroup
}

// ResctrlGroup keeps all info for one configured class in /sys/fs/resctrl.
type ResctrlGroup struct {
	Name         string
	Path         string
	BmSchemata   string
	SizeSchemata string
	SizeKib      int
	CacheLevel   string
}

// ExcatBuffers provides an interface that can be implemented by the mock for unit tests
// and can thus replace reading values from the local filesystem by dummy values.
type ExcatBuffers interface {
	GetClassNames() ([]string, error)
	ReadFile(string, bool) ([]string, error)
}

// readFromFs reads configured classes from the resctrl pseudo filesystem.
// Gets all classes names, bitmask schemata and size schemata files.
func (r *Resctrl) readFromFs() error {
	// get class names
	names, err := r.ExcatBuffers.GetClassNames()
	if err != nil {
		return fmt.Errorf("error when reading in class names: %w", err)
	}

	numClasses := len(names)
	if numClasses == 0 {
		return fmt.Errorf("error in readFromFs, no classes detected in /sys/fs/resctrl")
	}

	r.ResctrlGroups = make([]ResctrlGroup, numClasses)

	// for each class
	for ind, name := range names {
		r.ResctrlGroups[ind].Name = name

		// get class paths
		switch name {
		case DefaultClass:
			r.ResctrlGroups[ind].Path = RdtctrlPath
		default:
			r.ResctrlGroups[ind].Path = path.Join(RdtctrlPath, name)
		}

		// read schemata file
		schemata, err := r.ExcatBuffers.ReadFile(path.Join(r.ResctrlGroups[ind].Path, "schemata"), true)
		if err != nil {
			return fmt.Errorf("error in readFromFs: %w", err)
		}

		if len(schemata) != 1 {
			return fmt.Errorf("multiple definitions in Bit Mask Schemata file, just one cache level allowed per buffer")
		}

		r.ResctrlGroups[ind].BmSchemata = schemata[0]

		// read size file
		schemata, err = r.ExcatBuffers.ReadFile(path.Join(r.ResctrlGroups[ind].Path, "size"), true)
		if err != nil {
			return fmt.Errorf("error in readFromFs: %w", err)
		}

		if len(schemata) != 1 {
			return fmt.Errorf("multiple definitions in Size Schemata file, just one cache level allowed per buffer")
		}

		r.ResctrlGroups[ind].SizeSchemata = schemata[0]
	}

	return nil
}

// GetClassNames reads class names of classes configured in /sys/fs/resctrl.
func (r *Resctrl) GetClassNames() ([]string, error) {
	// init rdt
	if err := rdt.Initialize(""); err != nil {
		return []string{}, fmt.Errorf("RDT not supported: %w", err)
	}

	// get class names
	ctrlgrp := rdt.GetClasses()

	// check if RDT CAT is supported
	numClasses := len(ctrlgrp)
	if numClasses == 0 {
		log.Info().Msgf("RDT CAT not supported or properly configured on this machine.")
		os.Exit(0)
	}

	log.Debug().Msgf("Detected %v classes.", numClasses)

	names := make([]string, numClasses)
	for ind, group := range ctrlgrp {
		names[ind] = group.Name()
	}

	return names, nil
}

// ReadFile reads in a file.
// For schemata files, it ensures the file contains just one line, i.e. only
// one cache level is utilized.
func (r *Resctrl) ReadFile(path string, isSchemata bool) ([]string, error) {
	// open file
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("error when trying to open %v. Details: %w", path, err)
	}
	defer file.Close()

	// set up scanner to walk through file line by line
	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	// vars for scanning file
	catDefined := false

	var (
		lines       []string
		keepLine    bool
		currentLine string
	)

	// loop over file rows
	for {
		// get next line
		success := scanner.Scan()
		if !success {
			if err := scanner.Err(); err != nil {
				return nil, fmt.Errorf("error when scanning %v. Details: %w", path, err)
			}

			break
		}

		currentLine = scanner.Text()

		// ensure only one cache level is assigned to buffer
		if isSchemata {
			catReg := regexp.MustCompile(`^\s*L\d:.+`)
			mbaReg := regexp.MustCompile(`^\s*MB:.+`)

			switch {
			case catReg.MatchString(currentLine) && !catDefined:
				catDefined = true
				keepLine = true

			case catReg.MatchString(currentLine) && catDefined:
				note := "only one cache level definition supported per class"

				return nil, fmt.Errorf("several definitions detected in %v: %v", path, note)

			case mbaReg.MatchString(currentLine):
				keepLine = false

			default:
				return nil, fmt.Errorf("unknown format in %v", path)
			}
		} else {
			keepLine = true
		}
		if keepLine {

			// rm all whitespace chars and eol
			currentLine = strings.ReplaceAll(currentLine, " ", "")
			currentLine = strings.ReplaceAll(currentLine, "\n", "")

			// read in scanned line
			lines = append(lines, currentLine)
		}
	}

	return lines, nil
}

// GetBufferPids reads out all PIDs assigned to a buffer.
// The provided path has to point to the according tasks file.
func (r *Resctrl) GetBufferPids(path string) ([]string, error) {
	log.Debug().Msgf("Get PIDs in %v.", path)

	pids, err := r.ExcatBuffers.ReadFile(path, false)
	if err != nil {
		return nil, fmt.Errorf("error in GetBufferPids: %w", err)
	}

	return pids, nil
}
