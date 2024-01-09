// Copyright (C) 2023 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"

	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog/log"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ExcatMutatePods struct for mutating excat pods
type ExcatMutatePods struct {
	decoder *admission.Decoder
	Log     zerologr.Logger
}

// Handle mutate excat pods with
func (excatMutate *ExcatMutatePods) Handle(ctx context.Context, req admission.Request) admission.Response {
	pod := &corev1.Pod{}
	logger := excatMutate.Log.WithValues("req", fmt.Sprintf("namespace: %s, name: %s", req.Namespace, req.Name))
	logger.Info("mutate pod")
	// Only mutate on create
	if req.Operation != "CREATE" {
		return admission.Allowed("")
	}

	if err := excatMutate.decoder.Decode(req, pod); err != nil {
		logger.Error(err, "Error decoding AdmissionRequest")

		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := mutateExcatPod(pod); err != nil {
		logger.Error(err, "Error mutating pod request")

		return admission.Errored(http.StatusInternalServerError, err)
	}

	marshaledPod, err := json.Marshal(pod)
	if err != nil {
		logger.Error(err, "failed to marshal pod")

		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshaledPod)
}

// InjectDecoder will be automatically called to inject decoder
func (excatMutate *ExcatMutatePods) InjectDecoder(d *admission.Decoder) error {
	excatMutate.decoder = d

	return nil
}

func mutateExcatPod(pod *corev1.Pod) error {
	if pod.Annotations == nil {
		log.Info().Msg("No annotations found")

		return nil
	}

	for annotationKey, annotationValue := range pod.GetAnnotations() {
		reg := regexp.MustCompile("^intel.com/excat-l[2,3]$")
		matched := reg.MatchString(annotationKey)

		if !matched {
			continue
		}

		intAnnotationValue, err := strconv.Atoi(annotationValue)
		if err != nil {
			return fmt.Errorf("error converting annotation value to int")
		}

		// Add affinity to pod
		if pod.Spec.Affinity == nil {
			pod.Spec.Affinity = &corev1.Affinity{}
		}

		if pod.Spec.Affinity.NodeAffinity == nil {
			pod.Spec.Affinity.NodeAffinity = &corev1.NodeAffinity{}
		}

		if pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
			pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
		}

		nodeSelectorRequirement := corev1.NodeSelectorRequirement{
			Key:      annotationKey,
			Operator: corev1.NodeSelectorOpGt,
			Values:   []string{strconv.Itoa(intAnnotationValue - 1)},
		}

		if len(pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms) > 0 {
			nodeSelectorTerms := pod.Spec.Affinity.NodeAffinity.
				RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
			for i := range nodeSelectorTerms {
				nst := &nodeSelectorTerms[i]
				nst.MatchExpressions = append(nst.MatchExpressions, nodeSelectorRequirement)
			}
		} else {
			pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.
				NodeSelectorTerms = []corev1.NodeSelectorTerm{
				{MatchExpressions: []corev1.NodeSelectorRequirement{nodeSelectorRequirement}},
			}
		}

		// Add resource requests and limits to pod
		for ind := range pod.Spec.Containers {
			resourceRequests := pod.Spec.Containers[ind].Resources.Requests
			resourceLimits := pod.Spec.Containers[ind].Resources.Limits
			// If "no" prior container resource Requests exist, then container resource Limits would not exist too.
			// So create new Requests and Limits resources
			if len(resourceRequests) == 0 {
				pod.Spec.Containers[ind].Resources = corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceName(annotationKey): resource.MustParse("1"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceName(annotationKey): resource.MustParse("1"),
					},
				}
			} else {
				// If container resource Requests already exist, then check if container resource Limits exist
				// If limits do not exist, then append Requests and create new Limits resource
				if len(resourceLimits) == 0 {
					resourceRequests[corev1.ResourceName(annotationKey)] = resource.MustParse("1")
					pod.Spec.Containers[ind].Resources = corev1.ResourceRequirements{
						Requests: pod.Spec.Containers[ind].Resources.Requests,
						Limits: corev1.ResourceList{
							corev1.ResourceName(annotationKey): resource.MustParse("1"),
						},
					}
				} else {
					// If both container resource Requests and Limits already exist, then append requests and limits
					resourceRequests[corev1.ResourceName(annotationKey)] = resource.MustParse("1")
					resourceLimits[corev1.ResourceName(annotationKey)] = resource.MustParse("1")
				}
			}
		}

		log.Info().Msg("pod mutated with annotation: " + annotationKey)
	}

	return nil
}
