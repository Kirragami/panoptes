package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/Kirragami/panoptes/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type EyeState struct {
	LastSeen time.Time
	Online   bool
}

type PanoptesServer struct {
	proto.UnimplementedPanoptesServiceServer

	mu        sync.Mutex
	eyes      map[string]EyeState
	chronicle *Chronicle
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

		s.mu.Lock()
		for eyeID, eye := range s.eyes {
			if eye.Online && eye.LastSeen.Before(cutoff) {
				eye.Online = false
				s.eyes[eyeID] = eye

				log.Printf("[PANOPTICON] Eye closed: %s", eyeID)
			}
		}
		s.mu.Unlock()
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

	log.Printf("[PANOPTICON] Received binding request from Eye ID: %s", eyeID)

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

		log.Printf(
			"[PANOPTICON] Vigil from Eye %s at %d",
			eyeID,
			pulse.GetSentAtUnix(),
		)

		signal := &proto.PanopticonSignal{
			Message: "Panopticon sees you, Eye " + eyeID,
		}

		if err := stream.Send(signal); err != nil {
			return fmt.Errorf("send Panopticon signal: %w", err)
		}
	}
}

func main() {
	chronicle, err := openChronicle("./panopticon.chronicle.db")
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

	panoptesServer := &PanoptesServer{
		eyes:      eyes,
		chronicle: chronicle,
	}

	go panoptesServer.watchForClosedEyes()

	// this is a mint for testing purposes, dont forget to remove mkayy?? :))
	seal, expiresAt, err := panoptesServer.issueSeal()
	if err != nil {
		log.Fatalf("Panopticon could not issue a test Seal: %v", err)
	}

	log.Printf(
		"[PANOPTICON] Test Seal minted (expires %s): %s",
		expiresAt.Format(time.RFC3339),
		seal,
	)

	port := ":50051"
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Panopticon failed to wake up: %v", err)
	}

	grpcServer := grpc.NewServer(
		grpc.Creds(aegis),
	)
	proto.RegisterPanoptesServiceServer(grpcServer, panoptesServer)

	fmt.Printf("Panopticon is watching on port %s...\n", port)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("The Iris has shattered: %v", err)
	}
}
