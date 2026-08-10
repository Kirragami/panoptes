package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"
)

func startControlPanel(panoptes *PanoptesServer) error {
	controlPanel, err := newControlPanelFromEnvironment(panoptes)
	if err != nil {
		return fmt.Errorf("configure control panel: %w", err)
	}
	if controlPanel == nil {
		return nil
	}

	listener, err := controlPanel.listen()
	if err != nil {
		return err
	}

	log.Printf(
		"[PANEL] Control panel listening on https://%s",
		controlPanel.address,
	)

	go func() {
		if err := controlPanel.serve(listener); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Panopticon control panel stopped: %v", err)
		}
	}()

	return nil
}

func panelTLSConfig() (*tls.Config, error) {
	certificate, err := loadPanopticonCertificate()
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{certificate},
		MinVersion:   tls.VersionTLS13,
		NextProtos:   []string{"h2", "http/1.1"},
	}, nil
}
