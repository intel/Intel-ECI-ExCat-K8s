// Copyright (C) 2023 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func getValidExcatPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:         "testpod",
			GenerateName: "testpod",
			Labels: map[string]string{
				"excat": "yes",
			},
			Annotations: map[string]string{
				"intel.com/excat-l2": "2560",
				"intel.com/excat-l3": "3087",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test",
					Image: "testImage",
					Command: []string{
						"sleep",
					},
				},
			},
		},
	}
}

func getMutatedExcatPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:         "testpod",
			GenerateName: "testpod",
			Labels: map[string]string{
				"excat": "yes",
			},
			Annotations: map[string]string{
				"intel.com/excat-l2": "2560",
				"intel.com/excat-l3": "3087",
			},
		},
		Spec: corev1.PodSpec{
			Affinity: &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
						NodeSelectorTerms: []corev1.NodeSelectorTerm{
							{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      "intel.com/excat-l2",
										Operator: corev1.NodeSelectorOpGt,
										Values:   []string{"2559"},
									},
									{
										Key:      "intel.com/excat-l3",
										Operator: corev1.NodeSelectorOpGt,
										Values:   []string{"3086"},
									},
								},
							},
						},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "test",
					Image: "testImage",
					Command: []string{
						"sleep",
					},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"intel.com/excat-l2": resource.MustParse("1"),
							"intel.com/excat-l3": resource.MustParse("1"),
						},
						Limits: corev1.ResourceList{
							"intel.com/excat-l2": resource.MustParse("1"),
							"intel.com/excat-l3": resource.MustParse("1"),
						},
					},
				},
			},
		},
	}
}
