package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/Kirragami/panoptes/panopticon/cli"
	"github.com/Kirragami/panoptes/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const panopticonGRPCPort = "50051"

type EyeState struct {
	LastSeen time.Time
	Online   bool
}

type PanoptesServer struct {
	proto.UnimplementedPanoptesServiceServer
	proto.UnimplementedPanopticonEdictServiceServer
	proto.UnimplementedPanoptesOmenServiceServer
	proto.UnimplementedPanoptesOracleServiceServer

	mu         sync.Mutex
	eyes       map[string]EyeState
	chronicle  *Chronicle
	edictToken string
	harbinger  Harbinger
	sealEvents *sealConsumptionHub
}

type sealConsumptionEvent struct {
	Kind   string `json:"kind"`
	SealID string `json:"seal_id"`
}

type sealConsumptionHub struct {
	mu          sync.Mutex
	subscribers map[chan sealConsumptionEvent]struct{}
}

func newSealConsumptionHub() *sealConsumptionHub {
	return &sealConsumptionHub{
		subscribers: make(map[chan sealConsumptionEvent]struct{}),
	}
}

func (hub *sealConsumptionHub) subscribe() (
	<-chan sealConsumptionEvent,
	func(),
) {
	subscriber := make(chan sealConsumptionEvent, 1)

	hub.mu.Lock()
	hub.subscribers[subscriber] = struct{}{}
	hub.mu.Unlock()

	return subscriber, func() {
		hub.mu.Lock()
		if _, found := hub.subscribers[subscriber]; found {
			delete(hub.subscribers, subscriber)
			close(subscriber)
		}
		hub.mu.Unlock()
	}
}

func (hub *sealConsumptionHub) publish(event sealConsumptionEvent) {
	hub.mu.Lock()
	defer hub.mu.Unlock()

	for subscriber := range hub.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}

func (s *PanoptesServer) recordSight(eyeID string) {
	seenAt := time.Now().UTC()

	sighting, isNew, err := s.chronicle.RecordSight(eyeID, seenAt)
	if err != nil {
		log.Printf("[PANOPTICON] Chronicle failed for Eye %s: %v", eyeID, err)
	}

	s.mu.Lock()
	eye, knownInMemory := s.eyes[eyeID]
	wasOnline := eye.Online

	eye.LastSeen = time.Now()
	eye.Online = true
	s.eyes[eyeID] = eye
	s.mu.Unlock()

	if isNew || !knownInMemory {
		log.Printf("[PANOPTICON] Eye first opened: %s", eyeID)
		return
	}

	if !wasOnline {
		log.Printf(
			"[PANOPTICON] Eye open again: %s (first seen %s)",
			eyeID,
			sighting.FirstSeen.Format(time.RFC3339),
		)
	}
}

func (s *PanoptesServer) watchForClosedEyes() {
	const pulseTimeout = 45 * time.Second

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		cutoff := time.Now().Add(-pulseTimeout)

		type closedEye struct {
			id       string
			lastSeen time.Time
		}

		var closed []closedEye

		s.mu.Lock()
		for eyeID, eye := range s.eyes {
			if eye.Online && eye.LastSeen.Before(cutoff) {
				eye.Online = false
				s.eyes[eyeID] = eye

				closed = append(closed, closedEye{
					id:       eyeID,
					lastSeen: eye.LastSeen,
				})

				log.Printf("[PANOPTICON] Eye closed: %s", eyeID)
			}
		}
		s.mu.Unlock()

		for _, eye := range closed {
			s.raiseEclipse(eye.id, eye.lastSeen)
		}
	}
}

func (s *PanoptesServer) BindEye(
	ctx context.Context,
	req *proto.BindRequest,
) (*proto.BindResponse, error) {
	eyeID := strings.TrimSpace(req.GetEyeId())

	if eyeID == "" {
		return &proto.BindResponse{
			Success:       false,
			StatusMessage: "eye_id is required",
		}, nil
	}

	brand, err := s.admitBind(eyeID, req.GetSeal(), req.GetBrand())
	if err != nil {
		log.Printf("[PANOPTICON] Bind refused for Eye %s: %v", eyeID, err)

		return &proto.BindResponse{
			Success:       false,
			StatusMessage: err.Error(),
		}, nil
	}

	if err := s.rememberRevelations(
		eyeID,
		req.GetVisions(),
	); err != nil {
		log.Printf(
			// ---- not me giggling to this chuunibyou ahh naming scheme of mine lmao
			"[PANOPTICON] Failed to remember Visions of the Eye %s: %v",
			eyeID,
			err,
		)
	}

	s.recordSight(eyeID)

	if err := s.chronicle.RememberEpithet(eyeID, req.GetEpithet()); err != nil {
		log.Printf("[PANOPTICON] Bind refused Epithet for Eye %s: %v", eyeID, err)

		return &proto.BindResponse{
			Success:       false,
			StatusMessage: err.Error(),
		}, nil
	}

	log.Printf(
		"[PANOPTICON] Received binding request from Eye %s (%s)",
		req.GetEpithet(),
		eyeID,
	)

	return &proto.BindResponse{
		Success:       true,
		StatusMessage: "Bound successfully to the Panopticon",
		Brand:         brand,
	}, nil
}

func (s *PanoptesServer) KeepVigil(
	stream grpc.BidiStreamingServer[proto.EyePulse, proto.PanopticonSignal],
) error {
	for {
		pulse, err := stream.Recv()
		if err == io.EOF {
			log.Printf("[PANOPTICON] An eye has closed")
			return nil
		}

		if err != nil {
			return fmt.Errorf("receive Eye pulse: %w", err)
		}

		eyeID := strings.TrimSpace(pulse.GetEyeId())
		if eyeID == "" {
			return status.Error(codes.InvalidArgument, "eye_id is required")
		}

		brandHash, branded, err := s.chronicle.RecallBrandHash(eyeID)
		if err != nil {
			return status.Error(
				codes.Internal,
				"Panopticon could not recall the Eye's Brand",
			)
		}

		if !branded || !matchesBrand(brandHash, pulse.GetBrand()) {
			log.Printf("[PANOPTICON] Vigil refused for Eye %s: invalid Brand", eyeID)

			return status.Error(
				codes.Unauthenticated,
				"valid Brand is required to keep Vigil",
			)
		}

		s.recordSight(eyeID)

		rememberedGazes, err := s.chronicle.RecallGazes(eyeID)
		if err != nil {
			log.Printf(
				"[PANOPTICON] Failed to recall Gazes for Eye %s: %v",
				eyeID,
				err,
			)

			rememberedGazes = nil
		}

		gazes := make([]*proto.Gaze, 0, len(rememberedGazes))

		for _, remembered := range rememberedGazes {
			gazes = append(gazes, revealGaze(remembered))
		}

		log.Printf(
			"[PANOPTICON] Vigil from Eye %s at %v",
			eyeID,
			pulse.GetSentAtUnix(),
		)

		// gaze configs will be send over vigil for now
		// change it to instant send to eye in later versions kay?
		// not to mention the strain on db for this :'))
		signal := &proto.PanopticonSignal{
			Message: "Panopticon sees you, Eye " + eyeID,
			Gazes:   gazes,
		}

		if err := stream.Send(signal); err != nil {
			return fmt.Errorf("send Panopticon signal: %w", err)
		}
	}
}

func main() {
	if handled, err := cli.Handle(os.Args[1:]); handled {
		if err != nil {
			fmt.Fprintf(os.Stderr, "panopticon: %v\n", err)
			os.Exit(1)
		}
		return
	}

	chroniclePath, err := resolveChroniclePath()
	if err != nil {
		log.Fatalf("Panopticon could not place its Chronicle: %v", err)
	}

	chronicle, err := openChronicle(chroniclePath)
	if err != nil {
		log.Fatalf("Panopticon could not open its Chronicle: %v", err)
	}
	defer chronicle.Close()

	sightings, err := chronicle.RecallSightings()
	if err != nil {
		log.Fatalf("Panopticon could not recall sightings: %v", err)
	}

	eyes := make(map[string]EyeState, len(sightings))
	for _, sighting := range sightings {
		eyes[sighting.EyeID] = EyeState{
			LastSeen: sighting.LastSeen,
			Online:   false,
		}
	}

	log.Printf("[PANOPTICON] Recalled %d Eyes from the Chronicle", len(sightings))

	aegis, err := raiseAegis()
	if err != nil {
		log.Fatalf("Panopticon could not raise its Aegis: %v", err)
	}

	edictToken := strings.TrimSpace(
		os.Getenv("PANOPTICON_EDICT_TOKEN"),
	)

	if len(edictToken) < 32 {
		log.Fatalf("PANOPTICON_EDICT_TOKEN must be at least 32 characters")
	}

	if edictToken == "" {
		log.Fatalf("PANOPTICON_EDICT_TOKEN is required")
	}

	harbinger, err := awakenHarbinger(
		context.Background(),
		chronicle,
	)
	if err != nil {
		log.Fatalf("Panopticon could not awaken its Harbinger: %v", err)
	}


	panoptesServer := &PanoptesServer{
		eyes:       eyes,
		chronicle:  chronicle,
		edictToken: edictToken,
		harbinger:  harbinger,
		sealEvents: newSealConsumptionHub(),
	}

	go panoptesServer.watchForClosedEyes()

	err = startControlPanel(panoptesServer)
	if err != nil {
		log.Fatalf("Panopticon could not start its control panel: %v", err)
	}

	port := ":" + panopticonGRPCPort
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Panopticon failed to wake up: %v", err)
	}

	grpcServer := grpc.NewServer(
		grpc.Creds(aegis),
	)
	proto.RegisterPanoptesServiceServer(grpcServer, panoptesServer)
	proto.RegisterPanopticonEdictServiceServer(
		grpcServer,
		panoptesServer,
	)
	proto.RegisterPanoptesOmenServiceServer(
		grpcServer,
		panoptesServer,
	)
	proto.RegisterPanoptesOracleServiceServer(
		grpcServer,
		panoptesServer,
	)

	fmt.Printf("Panopticon is watching on port %s...\n", port)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("The Iris has shattered: %v", err)
	}
}
