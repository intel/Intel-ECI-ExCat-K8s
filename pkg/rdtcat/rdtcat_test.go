// Copyright (C) 2023 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package rdtcat_test

import (
	"path"

	"github.com/csl-svc/excat/pkg/mocks"
	"github.com/csl-svc/excat/pkg/rdtcat"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Rdtcat", func() {
	var (
		resctrl             *rdtcat.Resctrl
		rdtcatBuffers       *rdtcat.Buffers
		mockCtrl            *gomock.Controller
		mockExcatBuffers    *mocks.MockExcatBuffers
		PathDefault         string
		PathClass0          string
		PathClass1          string
		BmSchemataClass0    []string
		BmSchemataClass1    []string
		BmSchemataDefault   []string
		SizeSchemataClass0  []string
		SizeSchemataClass1  []string
		SizeSchemataDefault []string
		CacheLevelClass0    string
		CacheLevelClass1    string
		CacheLevelDefault   string
		SizeKibClass0       int
		SizeKibClass1       int
		SizeKibDefault      int
		DpL2Label           string
		DpL3Label           string
		tasks               []string
		ClassNames          []string
		path2tasks          string
	)

	// initialize
	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockExcatBuffers = mocks.NewMockExcatBuffers(mockCtrl)
	})

	// cleanup
	AfterEach(func() {
		// check if all registered mocks were called
		mockCtrl.Finish()
	})

	Context("When reading /sys/fs/resctrl with class0 and class1 being configured", func() {
		BeforeEach(func() {
			// dummy input values (usually read from /sys/fs/resctrl)
			resctrl = &rdtcat.Resctrl{
				ExcatBuffers: mockExcatBuffers,
			}
			rdtcatBuffers = &rdtcat.Buffers{
				Resctrl: *resctrl,
			}
			BmSchemataClass0 = []string{"L3:0=00003"}
			BmSchemataClass1 = []string{"L3:0=0001c"}
			BmSchemataDefault = []string{"L3:0=f0000"}
			SizeSchemataClass0 = []string{"L3:0=2621440"}
			SizeSchemataClass1 = []string{"L3:0=3932160"}
			SizeSchemataDefault = []string{"L3:0=5242880"}
			ClassNames = []string{"class0", "class1", rdtcat.DefaultClass}

			// expected extracted and computed values
			PathDefault = rdtcat.RdtctrlPath
			PathClass0 = rdtcat.RdtctrlPath + "/class0"
			PathClass1 = rdtcat.RdtctrlPath + "/class1"
			CacheLevelClass0 = "L3"
			CacheLevelClass1 = "L3"
			CacheLevelDefault = "L3"
			SizeKibClass0 = 2560
			SizeKibClass1 = 3840
			SizeKibDefault = 5120

			// mock expectations
			mockExcatBuffers.EXPECT().GetClassNames().Return(ClassNames, nil).Times(1)
			mockExcatBuffers.EXPECT().ReadFile(path.Join(PathDefault, "schemata"), true).
				Return(BmSchemataDefault, nil).Times(1)
			mockExcatBuffers.EXPECT().ReadFile(path.Join(PathClass0, "schemata"), true).
				Return(BmSchemataClass0, nil).Times(1)
			mockExcatBuffers.EXPECT().ReadFile(path.Join(PathClass1, "schemata"), true).
				Return(BmSchemataClass1, nil).Times(1)
			mockExcatBuffers.EXPECT().ReadFile(path.Join(PathDefault, "size"), true).
				Return(SizeSchemataDefault, nil).Times(1)
			mockExcatBuffers.EXPECT().ReadFile(path.Join(PathClass0, "size"), true).
				Return(SizeSchemataClass0, nil).Times(1)
			mockExcatBuffers.EXPECT().ReadFile(path.Join(PathClass1, "size"), true).
				Return(SizeSchemataClass1, nil).Times(1)
		})

		It("should collect schemata and extract cache level and size in KiB", func() {
			Expect(rdtcatBuffers.GetAllBuffers()).To(BeNil())
			Expect(rdtcatBuffers.Resctrl.ResctrlGroups[0].Path).To(Equal(PathClass0))
			Expect(rdtcatBuffers.Resctrl.ResctrlGroups[1].Path).To(Equal(PathClass1))
			Expect(rdtcatBuffers.Resctrl.ResctrlGroups[2].Path).To(Equal(PathDefault))
			Expect(rdtcatBuffers.Resctrl.ResctrlGroups[0].BmSchemata).To(Equal(BmSchemataClass0[0]))
			Expect(rdtcatBuffers.Resctrl.ResctrlGroups[1].BmSchemata).To(Equal(BmSchemataClass1[0]))
			Expect(rdtcatBuffers.Resctrl.ResctrlGroups[2].BmSchemata).To(Equal(BmSchemataDefault[0]))
			Expect(rdtcatBuffers.Resctrl.ResctrlGroups[0].SizeSchemata).To(Equal(SizeSchemataClass0[0]))
			Expect(rdtcatBuffers.Resctrl.ResctrlGroups[1].SizeSchemata).To(Equal(SizeSchemataClass1[0]))
			Expect(rdtcatBuffers.Resctrl.ResctrlGroups[2].SizeSchemata).To(Equal(SizeSchemataDefault[0]))
			Expect(rdtcatBuffers.Resctrl.ResctrlGroups[0].CacheLevel).To(Equal(CacheLevelClass0))
			Expect(rdtcatBuffers.Resctrl.ResctrlGroups[1].CacheLevel).To(Equal(CacheLevelClass1))
			Expect(rdtcatBuffers.Resctrl.ResctrlGroups[2].CacheLevel).To(Equal(CacheLevelDefault))
			Expect(rdtcatBuffers.Resctrl.ResctrlGroups[0].SizeKib).To(Equal(SizeKibClass0))
			Expect(rdtcatBuffers.Resctrl.ResctrlGroups[1].SizeKib).To(Equal(SizeKibClass1))
			Expect(rdtcatBuffers.Resctrl.ResctrlGroups[2].SizeKib).To(Equal(SizeKibDefault))
		})
	})

	Context("When reading PIDs out of /sys/fs/resctrl/class0/tasks", func() {
		BeforeEach(func() {
			// dummy input values (usually read from /sys/fs/resctrl/class0/tasks)
			path2tasks = "/sys/fs/resctrl/class0/tasks"

			// expected extracted and computed values
			tasks = []string{"10", "11", "12"}

			// mock expectations
			mockExcatBuffers.EXPECT().ReadFile(path2tasks, false).
				Return(tasks, nil).Times(1)
		})

		It("should output the PIDs assigned to the buffer", func() {
			resctrl = &rdtcat.Resctrl{
				ExcatBuffers: mockExcatBuffers,
			}
			Expect(resctrl.GetBufferPids(path2tasks)).To(Equal(tasks))
		})
	})

	Context("When creating labels for all buffers", func() {
		BeforeEach(func() {
			// input values
			resctrl = &rdtcat.Resctrl{
				ResctrlGroups: []rdtcat.ResctrlGroup{
					{
						Name:       "class0",
						SizeKib:    2560,
						CacheLevel: "L3",
					},
					{
						Name:       "class1",
						SizeKib:    3840,
						CacheLevel: "L3",
					},
					{
						Name:       rdtcat.DefaultClass,
						SizeKib:    5120,
						CacheLevel: "L3",
					},
				},
			}
			rdtcatBuffers = &rdtcat.Buffers{
				Resctrl: *resctrl,
			}

			// expected values
			DpL2Label = ""
			DpL3Label = "2560"
		})

		It("should create device plugin labels for level 2 and 3", func() {
			Expect(rdtcatBuffers.CreateLabels()).To(BeNil())
			Expect(rdtcatBuffers.DpL2Label).To(Equal(DpL2Label))
			Expect(rdtcatBuffers.DpL3Label).To(Equal(DpL3Label))
		})
	})

	Context("When filtering out buffers based on cache level", func() {
		BeforeEach(func() {
			// input values
			resctrl = &rdtcat.Resctrl{
				ResctrlGroups: []rdtcat.ResctrlGroup{
					{
						Name:         "class0",
						Path:         rdtcat.RdtctrlPath + "/class0",
						BmSchemata:   "BmSchemataClass0",
						SizeSchemata: "SizeKibClass0",
						SizeKib:      0,
						CacheLevel:   "L2",
					},
					{
						Name:         "class1",
						Path:         rdtcat.RdtctrlPath + "/class1",
						BmSchemata:   "BmSchemataClass1",
						SizeSchemata: "SizeKibClass10",
						SizeKib:      1,
						CacheLevel:   "L2",
					},
					{
						Name:         "class3",
						Path:         rdtcat.RdtctrlPath + "/class3",
						BmSchemata:   "BmSchemataClass3",
						SizeSchemata: "SizeKibClass3",
						SizeKib:      3,
						CacheLevel:   "L3",
					},
					{
						Name:         "class4",
						Path:         rdtcat.RdtctrlPath + "/class4",
						BmSchemata:   "BmSchemataClass4",
						SizeSchemata: "SizeKibClass4",
						SizeKib:      4,
						CacheLevel:   "L2",
					},
					{
						Name:         "class5",
						Path:         rdtcat.RdtctrlPath + "/class5",
						BmSchemata:   "BmSchemataClass5",
						SizeSchemata: "SizeKibClass5",
						SizeKib:      5,
						CacheLevel:   "L3",
					},
				},
			}
			rdtcatBuffers = &rdtcat.Buffers{
				Resctrl: *resctrl,
			}
		})

		It("should extract cache level 3 buffers", func() {
			// expected values
			l3Buffers := rdtcat.Resctrl{
				ResctrlGroups: []rdtcat.ResctrlGroup{
					{
						Name:         "class3",
						Path:         rdtcat.RdtctrlPath + "/class3",
						BmSchemata:   "BmSchemataClass3",
						SizeSchemata: "SizeKibClass3",
						SizeKib:      3,
						CacheLevel:   "L3",
					},
					{
						Name:         "class5",
						Path:         rdtcat.RdtctrlPath + "/class5",
						BmSchemata:   "BmSchemataClass5",
						SizeSchemata: "SizeKibClass5",
						SizeKib:      5,
						CacheLevel:   "L3",
					},
				},
			}
			l3Bufs := rdtcat.Buffers{
				Resctrl: l3Buffers,
			}

			Expect(rdtcatBuffers.ExtractBuffers(3)).To(Equal(&l3Bufs))
		})
	})
})
