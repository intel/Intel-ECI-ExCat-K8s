// Copyright (C) 2023 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package rdtcat_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestRdtcat(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Rdtcat Suite")
}
