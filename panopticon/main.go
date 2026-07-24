package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"
	"github.com/Kirragami/panoptes/proto"
)

type PanoptesServer struct {
	proto.UnimplementedPanoptesServiceServer
}

func (s *PanoptesServer) BindEye(ctx context.Context, req *proto.BindRequest) (*proto.BindResponse, error) {
	eyeID := req.GetEyeId()
	log.Printf("[PANOPTICON] Received binding request from Eye ID: %s", eyeID)
	return &proto.BindResponse {
		Success: true,
		StatusMessage: "Bound successfully to the Panopticon",
	}, nil
}

func main() {
	port := "50051"
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("Panopticon failed to wake up: %v", err)
	}

	grpcServer := grpc.NewServer()
	proto.RegisterPanoptesServiceServer(grpcServer, &PanoptesServer{})
	fmt.Printf("Panopticon is watching on port %s...\n", port)

	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("The Iris has shattered: %v", err)
	}
}