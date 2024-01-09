// Copyright (C) 2023 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ = Describe("excat pod mutation", func() {
	Describe("Handle requests", func() {
		Context("Not a create request", func() {
			It("should ignore request", func() {
				excatMutate := ExcatMutatePods{
					decoder: &admission.Decoder{},
					Log:     ctrl.Log.WithName("excatAdmission"),
				}
				ctx := context.Background()
				req := admission.Request{}
				req.Operation = "OTHER"
				res := excatMutate.Handle(ctx, req)
				Expect(res.AdmissionResponse.Allowed).To(BeTrue())
			})
		})

		Context("empty request", func() {
			It("should reject request", func() {
				excatMutate := ExcatMutatePods{
					decoder: &admission.Decoder{},
					Log:     ctrl.Log.WithName("test"),
				}
				ctx := context.Background()
				req := admission.Request{}
				req.Operation = "CREATE"
				res := excatMutate.Handle(ctx, req)
				Expect(res.AdmissionResponse.Allowed).To(BeFalse())
			})
		})
	})

	Describe("mutate an Excat Pod ", func() {
		var pod, expectedPod *corev1.Pod
		BeforeEach(func() {
			pod = getValidExcatPod()
			expectedPod = getMutatedExcatPod()
		})
		Context("valid excat pod", func() {
			It("should mutate pod", func() {
				err := mutateExcatPod(pod)
				Expect(err).To(BeNil())
				Expect(pod.Spec.Affinity).To(Equal(expectedPod.Spec.Affinity), "Affinity should have been set")
				Expect(pod.Spec.Containers[0].Resources).To(Equal(expectedPod.Spec.Containers[0].Resources),
					"Container resource requests and limits should have been set")
			})
		})
		Context("already mutated pod", func() {
			It("should not return error ie ignore pod", func() {
				Expect(mutateExcatPod(expectedPod)).Should(Succeed())
			})
		})
	})
})
