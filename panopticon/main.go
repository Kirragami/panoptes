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

	mu   sync.Mutex
	eyes map[string]EyeState
}

func (s *PanoptesServer) recordSight(eyeID string) {
	s.mu.Lock()
	eye, known := s.eyes[eyeID]
	wasOnline := eye.Online

	eye.LastSeen = time.Now()
	eye.Online = true
	s.eyes[eyeID] = eye
	s.mu.Unlock()

	if !known {
		log.Printf("[PANOPTICON] Eye first opened: %s", eyeID)
		return
	}

	if !wasOnline {
		log.Printf("[PANOPTICON] Eye open again: %s", eyeID)
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

	s.recordSight(eyeID)

	log.Printf("[PANOPTICON] Received binding request from Eye ID: %s", eyeID)

	return &proto.BindResponse{
		Success:       true,
		StatusMessage: "Bound successfully to the Panopticon",
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
	port := ":50051"
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Panopticon failed to wake up: %v", err)
	}

	panoptesServer := &PanoptesServer{
		eyes: make(map[string]EyeState),
	}

	go panoptesServer.watchForClosedEyes()

	grpcServer := grpc.NewServer()
	proto.RegisterPanoptesServiceServer(grpcServer, panoptesServer)

	fmt.Printf("Panopticon is watching on port %s...\n", port)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("The Iris has shattered: %v", err)
	}
}
