// Copyright (C) 2023 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

/*
Package rdtcat implements functions to read RDT CAT configurations.

Classes configured in /sys/fs/resctrl are read and size as well as type of the
utilized cache are extracted.
*/
package rdtcat

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Buffers keeps all infos of configured classes within /sys/fs/resctrl
// as well as labels for the device plugin.
type Buffers struct {
	Resctrl
	DpL2Label string
	DpL3Label string
}

// InitLogger initializes the logger.
func InitLogger(ll zerolog.Level) {
	zerolog.SetGlobalLevel(ll)
	log.Info().Msgf("Rdtcat Logging level = %v.", ll)

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
}

// GetAllBuffers reads in all configured buffers in /sys/fs/resctrl.
// First all buffer details are read and then buffer sizes and cache levels are
// extracted out of the schema files.
func (r *Buffers) GetAllBuffers() error {
	// get available buffers
	log.Debug().Msg("Reading from /sys/fs/resctrl")
	if err := r.readFromFs(); err != nil { //nolint:wsl // ok to cuddle debug message
		return fmt.Errorf("error when reading buffers from /sys/fs/resctrl. Details: %w", err)
	}

	// extract buffer sizes and cache level
	log.Debug().Msg("Extracting buffer details")
	if err := r.extractBufferDetails(); err != nil { //nolint:wsl // ok to cuddle debug message
		return fmt.Errorf("error when extracting buffers sizes and cache level. Details: %w", err)
	}

	return nil
}

// ExtractBuffers extracts buffers that utilize cache of a given cache level.
func (r *Buffers) ExtractBuffers(cacheLevel int) *Buffers {
	log.Debug().Msgf("Filter buffers based on cacheLevel = %v.", cacheLevel)
	filter := fmt.Sprintf("L%v", cacheLevel)
	buffers := Buffers{}

	for _, group := range r.ResctrlGroups {
		if group.CacheLevel == filter {
			buffers.ResctrlGroups = append(buffers.ResctrlGroups, ResctrlGroup{
				Name:         group.Name,
				Path:         group.Path,
				BmSchemata:   group.BmSchemata,
				SizeSchemata: group.SizeSchemata,
				SizeKib:      group.SizeKib,
				CacheLevel:   group.CacheLevel,
			})
		}
	}

	return &buffers
}

// extractBufferDetails extracts buffer types (cache level) and sizes in KiB.
// Ensures that all cache IDs are configured the same so that all CPUs can
// access all buffers.
func (r *Buffers) extractBufferDetails() error {
	for ind, group := range r.ResctrlGroups {
		var sizes []string

		// split into cache Level and size info
		const sizeSchemataParts = 2

		sizeSchemataSplit := strings.Split(group.SizeSchemata, ":")
		if len(sizeSchemataSplit) != sizeSchemataParts {
			return fmt.Errorf("format error in %v", group.Path)
		}

		r.ResctrlGroups[ind].CacheLevel = sizeSchemataSplit[0]

		// split cache IDs
		ids := strings.Split(sizeSchemataSplit[1], ";")

		// extract sizes and ensure all cache IDs have been configured in the same way
		sizeReg := regexp.MustCompile(`\d+$`)
		for currentID := 0; currentID < len(ids); currentID++ {
			sizes = append(sizes, sizeReg.FindString(ids[currentID]))
			if currentID > 0 {
				if sizes[currentID] != sizes[currentID-1] {
					log.Info().Msgf("Different buffer sizes (%v and %v) detected for cache Level %v."+
						"Smallest size is used for all ExCAT buffers and thus the size difference to the bigger buffer is wasted. "+
						"One root cause can be if TCC is used and SW RAM buffer is allocated on one of the Level %v buffers.",
						sizes[currentID], sizes[currentID-1], r.ResctrlGroups[ind].CacheLevel, r.ResctrlGroups[ind].CacheLevel)
				}
			}
		}

		if len(sizes) == 0 {
			return fmt.Errorf("missing size info in %v", group.Path)
		}

		// get sizes in KiB
		var sizeBytes int
		if _, err := fmt.Sscanf(sizes[0], "%d", &sizeBytes); err != nil {
			return fmt.Errorf("error when casting size string to int. Details: %w", err)
		}

		r.ResctrlGroups[ind].SizeKib = bytes2kib(sizeBytes)
	}

	return nil
}

// CreateLabels creates labels to be used with the device plugin.
// Based on the labels, worker nodes can be patched to provide the size info
// in addition to the extended resources advertized by the device plugin.
// Only one label is created for L2 and L3 cache, respectively. Classes must
// only configure either L2 or L3 cache. If buffers for one cache level have
// different sizes, the smallest size is advertized.
func (r *Buffers) CreateLabels() error {
	var prevL2Size, prevL3Size int = -1, -1

	// Create single device plugin label for L2 and L3 with size in KiB, respectively.
	for _, group := range r.ResctrlGroups {
		if group.Name != DefaultClass {
			switch group.CacheLevel {
			case "L2":
				if prevL2Size == -1 {
					r.DpL2Label = fmt.Sprintf("%v", group.SizeKib)
				} else if group.SizeKib < prevL2Size {
					r.DpL2Label = fmt.Sprintf("%v", group.SizeKib)
				}

				prevL2Size = group.SizeKib

			case "L3":
				if prevL3Size == -1 {
					r.DpL3Label = fmt.Sprintf("%v", group.SizeKib)
				} else if group.SizeKib < prevL3Size {
					r.DpL3Label = fmt.Sprintf("%v", group.SizeKib)
				}

				prevL3Size = group.SizeKib
			}
		}
	}

	if r.DpL2Label == "" && r.DpL3Label == "" {
		return fmt.Errorf("no labels were created: configure resctrl and read in buffers first")
	}

	log.Debug().Msgf("L2 Label: %v", r.DpL2Label)
	log.Debug().Msgf("L3 Label: %v", r.DpL3Label)

	return nil
}

// bytes2kib converts bytes into kibibytes (KiB).
func bytes2kib(b int) int {
	return (b / 1024) //nolint:gomnd // no magic number in this case
}
