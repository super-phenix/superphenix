package api

import (
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/server"

	"github.com/rs/zerolog/log"
)

// StartAPI
//
//	@title									Superphenix API
//	@version								0.1
//	@description							Superphenix HTTP API
//
//	@contact.name							API Support
//	@contact.url							https://superphenix.net
//	@contact.email							contact@superphenix.net
//
//	@BasePath								/api
//
//	@securityDefinitions.apiKey				Kratos
//	@in										header
//	@name									ory_kratos_session
//	@description							Session cookie for authentication service Kratos. Used to get Refresh and Access Token used in Superphenix API
//
//	@securityDefinitions.apiKey				RefreshToken
//	@in										header
//	@name									session
//	@description							Refresh token with longer validity. Used to generate a new access token within the validity of the token.
//
//	@securityDefinitions.apiKey				Bearer
//	@in										header
//	@name									Authorization
//	@description							Access token used to check user access in Superphenix API.
//	@scope.OrganizationRead					Grants read access to organization
//	@scope.OrganizationWrite				Grants write access to organization
//	@scope.OrganizationIAMRead				Grants read access to organization IAM
//	@scope.OrganizationIAMWrite				Grants write access to organization IAM
//	@scope.OrganizationBillingRead			Grants read access to organization billing
//	@scope.OrganizationBillingWrite			Grants write access to organization billing
//	@scope.OrganizationProjectManagement	Grants project management access in organization
//	@scope.ProjectInstanceRead				Grants read access to instance in project
//	@scope.ProjectInstanceTerminal			Grants terminal access to instance in project
//	@scope.ProjectInstanceControl			Grants control (start, stop) access to instance in project
//	@scope.ProjectInstanceWrite				Grants write access to instance in project
//	@scope.ProjectSnapshotRead				Grants read access to Snapshot and Instance Snapshot in project
//	@scope.ProjectSnapshotWrite				Grants write access to Snapshot and Instance Snapshot in project
//	@scope.ProjectDiskRead					Grants read access to Disk in project
//	@scope.ProjectDiskWrite					Grants write access to Disk in project
//	@scope.ProjectVPCRead					Grants read access to VPC in project
//	@scope.ProjectVPCWrite					Grants write access to VPC in project
//	@scope.ProjectSubnetRead				Grants read access to Subnet in project
//	@scope.ProjectSubnetWrite				Grants write access to Subnet in project
//	@scope.ProjectEipRead					Grants read access to EIP in project
//	@scope.ProjectEipWrite					Grants write access to EIP in project
//	@scope.ProjectEipRead					Grants read access to LoadBalancer in project
//	@scope.ProjectEipWrite					Grants write access to LoadBalancer in project
//	@scope.ProjectSSHRead					Grants read access to SSH in project
//	@scope.ProjectSSHWrite					Grants write access to SSH in project

func StartAPI() {
	log.Debug().Msg("Starting APIs")
	go startAdminHTTP()
	go startHealthHTTP()
	startHTTP()
}

func startHTTP() {
	srv, err := server.InitializeServer(&config.Global)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Public HTTP server")
	}
	if err := srv.Run(config.Global.PublicHTTP.Address); err != nil {
		log.Fatal().Err(err).Msg("Failed to start Public HTTP endpoint")
	}
}

func startAdminHTTP() {
	if config.Global.AdminHTTP.Enabled {
		srv, err := server.InitializeAdminServer(&config.Global)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to initialize Admin HTTP server")
		}
		if err := srv.Run(config.Global.AdminHTTP.Address); err != nil {
			log.Fatal().Err(err).Msg("Failed to start Admin HTTP endpoint")
		}
	}
}

func startHealthHTTP() {
	if !config.Global.ReadinessProbe.Enabled {
		return
	}
	srv, err := server.InitializeHealthServer(&config.Global)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize Health HTTP server")
	}
	if err := srv.Run(config.Global.ReadinessProbe.Address); err != nil {
		log.Fatal().Err(err).Msg("Failed to start Health HTTP endpoint")
	}
}
