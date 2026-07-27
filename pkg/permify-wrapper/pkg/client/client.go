package client

import (
	"buf.build/gen/go/permifyco/permify/grpc/go/base/v1/basev1grpc"
	permifyGrpc "github.com/Permify/permify-go/grpc"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	Permission      basev1grpc.PermissionClient
	Schema          basev1grpc.SchemaClient
	Data            basev1grpc.DataClient
	Bundle          basev1grpc.BundleClient
	Tenancy         basev1grpc.TenancyClient
	Watch           basev1grpc.WatchClient
	clientConnected = false
)

// InitPermify initialize the connection with Permify Server
// As the communication go through gRPC, don't provide any schema in the endpoint
// Example endpoint : localhost:3478
func InitPermify(endpoint string) error {
	err := createClient(endpoint)

	if err != nil {
		log.Error().Err(err).Msg("Failed to create client client")
		return err
	}
	return nil
}

func createClient(endpoint string) error {
	c, err := permifyGrpc.NewClient(
		permifyGrpc.Config{
			Endpoint: endpoint,
		},
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	if err != nil {
		return err
	}

	Permission = c.Permission
	Schema = c.Schema
	Data = c.Data
	Bundle = c.Bundle
	Tenancy = c.Tenancy
	Watch = c.Watch
	clientConnected = true
	return nil
}

// IsConnected indicate if the client as been initialized correctly
func IsConnected() bool {
	if !clientConnected {
		log.Error().Msg("client not connected")
		return false
	} else {
		return true
	}
}
