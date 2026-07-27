package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/az"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/crud/product"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/proxy"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	logger "github.com/super-phenix/superphenix/pkg/utils/log"

	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/google/uuid"
)

var (
	FailedVpcCreation    = fmt.Errorf("failed to create VPC")
	FailedSubnetCreation = fmt.Errorf("failed to create Subnet")
)

func InitializeProjectDefaultResources(ctx context.Context, orgUUID, projectUUID uuid.UUID) error {
	log := logger.GetLogger(ctx)
	// Fetch AZ
	urls := az.FindAll(orgUUID.String())

	// For EACH AZ
	for i, url := range urls {
		log.Debug().Msgf("InitializeProjectDefaultResources - %d/%d generating default resource for az: %s", i+1, len(urls), url.Code)
		// Initialize VPC (in DB and Controller)
		if vpcEID, err := initializeVPC(ctx, orgUUID, projectUUID, url); err == nil {
			// If no error, Initialize Subnet (in DB and Controller)
			if err := initializeSubnet(ctx, orgUUID, projectUUID, url, vpcEID); err != nil {
				log.Err(err).Str("projectId", projectUUID.String()).Msg("InitializeProjectDefaultResources - Failed to initialize Subnet")
				return fmt.Errorf("InitializeProjectDefaultResources - Failed to initialize Subnet")
			}
		} else {
			log.Err(err).Str("projectId", projectUUID.String()).Msg("InitializeProjectDefaultResources - Failed to initialize VPC")
			return fmt.Errorf("InitializeProjectDefaultResources - Failed to initialize VPC")
		}
	}

	return nil
}

func initializeVPC(ctx context.Context, orgUUID, projectUUID uuid.UUID, az config.AZConfig) (string, error) {
	log := logger.GetLogger(ctx)
	vpc, err := product.Save(model.Product{
		ProductName:   config.Global.DefaultProducts.VPC.ProductName,
		CodeAZ:        az.Code,
		ProjectId:     projectUUID,
		ProductTypeId: model.ProductTypeVPC,
	})
	if err != nil {
		log.Err(err).Str("projectId", projectUUID.String()).Msg("Failed to save VPC in DB")
		return "", FailedVpcCreation
	}
	vpcId := vpc.ID
	m := spxId.Metadata{}
	err = m.GenerateMetadata(projectUUID.String(), orgUUID.String(), vpcId.String())
	if err != nil {
		log.Err(err).Str("projectId", projectUUID.String()).Str("productID", vpcId.String()).Msg("Failed to generate VPC metadata - cleaning Product")
		err := product.DeleteById(vpcId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to clean product")
		}

		return "", FailedVpcCreation
	}

	vpc.EffectiveID = m.GetResourceEffectiveID()
	vpc, err = product.Save(vpc)
	if err != nil {
		log.Err(err).Str("projectId", projectUUID.String()).Str("productID", vpcId.String()).Msg("Failed to save VPC EID - cleaning Product")
		err := product.DeleteById(vpcId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to clean product")
		}
		return "", FailedVpcCreation
	}

	// Send request to superphenix-controller
	body := CreateVPCSpxControllerBody{
		Metadata: m,
	}

	// update body
	marshal, err := json.Marshal(body)
	if err != nil {
		log.Err(err).Any("body", body).Msg("Failed to marshal VPC SPX Body - cleaning Product")
		err := product.DeleteById(vpcId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to clean product")
		}
		return "", FailedVpcCreation
	}

	// {base}/{orgaId}/{projectId}/vpc
	url := fmt.Sprintf("%s/%s/%s/vpc", az.ControllerUrl, orgUUID.String(), projectUUID.String())
	resp, err := proxy.SendRequest(ctx, url, "POST", bytes.NewReader(marshal), config.Global.Controller.AuthSecret)
	if err != nil {
		log.Err(err).Str("url", url).Msg("Failed to send request on controller")
		return "", FailedVpcCreation
	} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		log.Err(err).Str("projectId", projectUUID.String()).Str("url", url).Msg("Failed to create vpc on controller")
		return "", FailedVpcCreation
	}
	return vpc.EffectiveID, nil
}

func initializeSubnet(ctx context.Context, orgUUID, projectUUID uuid.UUID, az config.AZConfig, vpcEID string) error {
	log := logger.GetLogger(ctx)
	subnet, err := product.Save(model.Product{
		ProductName:   config.Global.DefaultProducts.Subnet.ProductName,
		CodeAZ:        az.Code,
		ProjectId:     projectUUID,
		ProductTypeId: model.ProductTypeSubnet,
	})
	if err != nil {
		log.Err(err).Str("projectId", projectUUID.String()).Msg("Failed to save Subnet in DB")
		return FailedSubnetCreation
	}

	subnetId := subnet.ID
	m := spxId.Metadata{}
	err = m.GenerateMetadata(projectUUID.String(), orgUUID.String(), subnetId.String())
	if err != nil {
		log.Err(err).Str("projectId", projectUUID.String()).Str("productID", subnetId.String()).Msg("Failed to generate Subnet metadata - cleaning Product")
		err := product.DeleteById(subnetId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to clean product")
		}
		return FailedSubnetCreation
	}

	subnet.EffectiveID = m.GetResourceEffectiveID()
	subnet, err = product.Save(subnet)
	if err != nil {
		log.Err(err).Str("projectId", projectUUID.String()).Str("productID", subnetId.String()).Msg("Failed to save Subnet EID - cleaning Product")
		err := product.DeleteById(subnetId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to clean product")
		}
		return FailedSubnetCreation
	}

	// Send request to superphenix-controller
	body := CreateSubnetSpxControllerBody{
		Metadata: m,
		General: struct {
			VpcEId string `json:"vpcEId"`
		}{
			VpcEId: vpcEID,
		},
		Network: struct {
			Private  bool   `json:"private"`
			Protocol string `json:"protocol"`
			IPv4     string `json:"ipv4,omitempty"`
			IPv6     string `json:"ipv6,omitempty"`
			DnsV4    string `json:"dnsV4,omitempty"`
			DnsV6    string `json:"dnsV6,omitempty"`
		}{
			Private:  config.Global.DefaultProducts.Subnet.Private,
			Protocol: config.Global.DefaultProducts.Subnet.Protocol,
			IPv4:     config.Global.DefaultProducts.Subnet.IPv4,
			IPv6:     config.Global.DefaultProducts.Subnet.IPv6,
		},
		NatGateway: struct {
			Enable bool `json:"enable"`
		}{
			Enable: config.Global.DefaultProducts.Subnet.NatGatewayEnabled,
		},
	}

	// update body
	marshal, err := json.Marshal(body)
	if err != nil {
		log.Err(err).Any("body", body).Msg("Failed to marshal Subnet SPX Body - cleaning Product")
		err := product.DeleteById(subnetId)
		if err != nil {
			log.Error().Err(err).Msg("Failed to clean product")
		}
		return FailedSubnetCreation

	}

	// {base}/{orgaId}/{projectId}/subnet
	url := fmt.Sprintf("%s/%s/%s/subnet", az.ControllerUrl, orgUUID.String(), projectUUID.String())
	resp, err := proxy.SendRequest(ctx, url, "POST", bytes.NewReader(marshal), config.Global.Controller.AuthSecret)
	if err != nil {
		log.Err(err).Str("url", url).Msg("Failed to send request on controller")
		return FailedSubnetCreation
	} else if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		log.Err(err).Str("projectId", projectUUID.String()).Str("url", url).Msg("Failed to create subnet on controller")
		return FailedSubnetCreation
	}
	return nil
}
