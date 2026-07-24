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

type PanoptesServer struct {
	proto.UnimplementedPanoptesServiceServer

	mu        sync.Mutex
	boundEyes map[string]time.Time
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

	s.mu.Lock()
	s.boundEyes[eyeID] = time.Now()
	s.mu.Unlock()

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

		s.mu.Lock()
		s.boundEyes[eyeID] = time.Now()
		s.mu.Unlock()

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
		boundEyes: make(map[string]time.Time),
	}

	grpcServer := grpc.NewServer()
	proto.RegisterPanoptesServiceServer(grpcServer, panoptesServer)

	fmt.Printf("Panopticon is watching on port %s...\n", port)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("The Iris has shattered: %v", err)
	}
}
