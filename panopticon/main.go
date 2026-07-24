package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/Kirragami/panoptes/proto"
	"google.golang.org/grpc"
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
