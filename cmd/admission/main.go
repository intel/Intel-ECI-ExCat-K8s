// Copyright (C) 2023 Intel Corporation
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"os"

	"github.com/csl-svc/excat/pkg/handler"
	"github.com/go-logr/zerologr"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	defaultPort            = 443
	tlsMinVersion          = "1.3"
	defaultCertFileName    = "tls.crt"
	defaultKeyFileName     = "tls.key"
	defaultCertDir         = "/run/secrets/tls"
	defaultHealthProbeAddr = ":8081"
)

func parseFlags(
	certFileName *string,
	keyFileName *string,
	certFilesDir *string,
	port *int,
	healthProbeBindAddress *string,
) {
	flag.StringVar(certFileName, "tls-cert-name", defaultCertFileName, ""+
		" x509 Certificate for HTTPS file name")
	flag.StringVar(keyFileName, "tls-private-key-name", defaultKeyFileName, ""+
		"x509 private key file name.")
	flag.StringVar(certFilesDir, "tls-cert-dir", defaultCertDir, ""+
		"directory containing certificate and private key files.")
	flag.IntVar(port, "port", defaultPort, ""+
		"Port used to access the admission controller")
	flag.StringVar(healthProbeBindAddress, "health-probe-bind-address", defaultHealthProbeAddr, ""+
		"address for healthz and readyz endpoint")

	flag.Parse()

	logger := zerolog.New(os.Stderr)
	logger = logger.With().Timestamp().Logger()

	log := zerologr.New(&logger)

	debug := flag.Bool("debug", false, "sets log level to debug")
	// Default level for this example is info, unless debug flag is present
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	ctrl.SetLogger(log)
}

func main() {
	var (
		certFileName           string
		keyFileName            string
		certFilesDir           string
		port                   int
		healthProbeBindAddress string
	)

	parseFlags(&certFileName, &keyFileName, &certFilesDir,
		&port,
		&healthProbeBindAddress)

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Port: port,
	})
	if err != nil {
		log.Error().Msg("unable to init manager")
		os.Exit(1)
	}

	webhookServer := mgr.GetWebhookServer()
	webhookServer.TLSMinVersion = tlsMinVersion
	webhookServer.CertDir = certFilesDir
	webhookServer.CertName = certFileName
	webhookServer.KeyName = keyFileName

	webhookServer.Register("/mutate", &webhook.Admission{
		Handler: admission.MultiMutatingHandler(&handler.ExcatMutatePods{Log: ctrl.Log.WithName("excatAdmission")}),
	})

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Error().Msg("unable to add healthz")
	}

	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Error().Msg("unable to add readyz")
	}

	log.Info().Str("cert dir", webhookServer.CertDir).
		Str("cert", webhookServer.CertName).
		Str("key", webhookServer.KeyName).
		Int("Port", webhookServer.Port).
		Msg("Starting with ...")

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Fatal().Err(err).Msgf("unable to start manager")
	}
}
