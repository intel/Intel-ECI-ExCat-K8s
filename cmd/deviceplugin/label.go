// Copyright (C) 2023 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/rs/zerolog/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// if ExCAT is deployed as a service within a cluster based on the provided helm chart,
// an InClusterConfig is used to patch node labels. If the device plugin is executed from
// outside a cluster (e.g. for debugging purposes), the kubeconfigPath can be used to
// provide the client config.
const (
	isInCluster          = true // true if deployed as service in cluster (default)
	rootResourceNameJSON = "intel.com~1"
	kubeconfigPath       = "/etc/rancher/rke2/rke2.yaml"
)

// rmAllLabels removes all ExCAT related labels.
func rmAllLabels() {
	rmNodeLabel(cacheLevel2, "")
	rmNodeLabel(cacheLevel3, "")
}

// addNodeLabel adds a label to a node.
func addNodeLabel(level int, labelValue string) error {
	labelKey := fmt.Sprintf("%v%v-l%v", rootResourceNameJSON, resourceBaseName, level)

	if err := patchNodeLabel("add", labelKey, labelValue); err != nil {
		return fmt.Errorf("error when adding node label: %w", err)
	}

	return nil
}

// rmNodeLabel removes a label from a node.
func rmNodeLabel(level int, labelValue string) {
	labelKey := fmt.Sprintf("%v%v-l%v", rootResourceNameJSON, resourceBaseName, level)

	err := patchNodeLabel("remove", labelKey, labelValue)
	if err != nil {
		log.Debug().Msgf("couldn't remove \"%v\", label either doesn't exist or there was an error: %v", labelKey, err)
	} else {
		log.Debug().Msgf("label \"%v\" successfully removed.", labelKey)
	}
}

// patchNodeLabel patches a node with a given label.
func patchNodeLabel(operation string, key string, value string) error {
	var (
		config *rest.Config
		err    error
	)

	if isInCluster {
		config, err = rest.InClusterConfig()
		if err != nil {
			return fmt.Errorf("in patchNodeLabel: %w", err)
		}
	} else {
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		loadingRules.ExplicitPath = kubeconfigPath

		configOverrides := &clientcmd.ConfigOverrides{}

		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		config, err = kubeConfig.ClientConfig()
		if err != nil {
			return fmt.Errorf("in patchNodeLabel: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("in patchNodeLabel: %w", err)
	}

	payload := []patchStringValue{{
		Op:    operation,
		Path:  fmt.Sprintf("/metadata/labels/%v", key),
		Value: value,
	}}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("in patchNodeLabel: %w", err)
	}

	var hostname string
	if isInCluster {
		hostname = os.Getenv("NODE_NAME")
	} else {
		hostname, err = os.Hostname()
		if err != nil {
			return fmt.Errorf("error when reading hostname: %w", err)
		}
	}

	_, err = clientset.CoreV1().Nodes().Patch(
		context.TODO(),
		hostname,
		types.JSONPatchType,
		payloadBytes,
		metav1.PatchOptions{})
	if err != nil {
		return fmt.Errorf("error when patching node: %w", err)
	}

	log.Debug().Msgf("Operation \"%v\" with label \"%v: %v\" successful.", operation, key, value)

	return nil
}
