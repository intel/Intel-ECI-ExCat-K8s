// Copyright (C) 2023 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestHandler(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Handler Suite")
}
