package main

import (
	"context"

	"github.com/Kirragami/panoptes/proto"
)

func (s *PanoptesServer) PairOracle(
	_ context.Context,
	request *proto.PairOracleRequest,
) (*proto.PairOracleResponse, error) {
	brand, err := s.pairOracle(
		request.GetOracleId(),
		request.GetOracleSeal(),
		request.GetFcmToken(),
	)
	if err != nil {
		return &proto.PairOracleResponse{
			Success:       false,
			StatusMessage: err.Error(),
		}, nil
	}

	return &proto.PairOracleResponse{
		Success:       true,
		StatusMessage: "Oracle paired",
		OracleBrand:   brand,
	}, nil
}

func (s *PanoptesServer) RefreshOracleToken(
	_ context.Context,
	request *proto.RefreshOracleTokenRequest,
) (*proto.RefreshOracleTokenResponse, error) {
	if err := s.refreshOracleToken(
		request.GetOracleId(),
		request.GetOracleBrand(),
		request.GetFcmToken(),
	); err != nil {
		return &proto.RefreshOracleTokenResponse{
			Success:       false,
			StatusMessage: err.Error(),
		}, nil
	}

	return &proto.RefreshOracleTokenResponse{
		Success:       true,
		StatusMessage: "Oracle token refreshed",
	}, nil
}
