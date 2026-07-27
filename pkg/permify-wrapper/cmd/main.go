package main

import (
	"context"
	"fmt"
	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg"
	v1 "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1"
	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/entity"
	basePermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/client"
	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/permission"

	"github.com/rs/zerolog/log"
)

const (
	organizationId = "f4597959-aedc-4c84-b359-b02695fde5ec"

	projectTest1 = "8932f745-859f-44be-b0ac-df2752549c01"
	projectTest2 = "d8e5b39c-ef37-498c-84a0-850a0e8db1fc"

	user1 = "dbc67fb8-d876-44cc-b896-2ad402633cec"
	user2 = "eea03b61-1b70-4d8b-b827-4e99a282d3a3"
	user3 = "c3d4a3ed-05ef-46b5-bd61-c36bc0f5db74"

	groupOwnerId     = "f8c75a55-464d-461f-aceb-239708619669"
	groupAdminId     = "2b2c983e-6119-407a-b219-ede8cbdfa137"
	groupBillingId   = "e6f37247-9178-4f47-ae43-6c6ccdbfd7e1"
	groupDeveloperId = "def3b6f5-1ae9-485a-bf14-38685fb6d1df"
)

var orgaPermissions = []string{basePermission.OrganizationRead,
	basePermission.OrganizationWrite,
	basePermission.OrganizationIAMRead,
	basePermission.OrganizationIAMWrite,
	basePermission.OrganizationBillingRead,
	basePermission.OrganizationBillingWrite,
	basePermission.OrganizationProjectManagement,
}
var projectPermissions = []string{
	basePermission.ProjectInstanceRead,
	basePermission.ProjectInstanceTerminal,
	basePermission.ProjectInstanceControl,
	basePermission.ProjectInstanceWrite,
}

var adminOrgaPermission = []bool{true, true, true, true, true, true, true}
var adminProjectPermission = []bool{true, true, true, true}

var billingOrgaPermission = []bool{true, false, false, false, true, true, false}
var billingProjectPermission = []bool{false, false, false, false}

var developerOrgaPermission = []bool{true, false, false, false, false, false, true}
var developerProjectPermission = []bool{true, true, true, true}

var noneProjectPermission = []bool{false, false, false, false}

func main() {
	ctx := context.Background()
	err := client.InitPermify("localhost:3478")
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to init Permify client")
	}

	// Clean
	_ = pkg.DeleteRelationOrganization(ctx, organizationId)

	if err := pkg.CreateRelationOrganization(ctx, pkg.Organization{
		Id:   organizationId,
		Name: "Orga Test",
	}, map[string][]string{
		groupOwnerId:     v1.DefaultOrganizationOwner,
		groupAdminId:     v1.DefaultOrganizationAdmin,
		groupBillingId:   v1.DefaultOrganizationBilling,
		groupDeveloperId: v1.DefaultOrganizationDeveloper,
	}); err != nil {
		log.Fatal().Err(err).Msg("Failed to init Organization")
	}

	if err := pkg.CreateRelationProject(ctx, organizationId, projectTest1, map[string][]string{
		groupOwnerId:     v1.DefaultProjectOwner,
		groupAdminId:     v1.DefaultProjectAdmin,
		groupBillingId:   v1.DefaultProjectBilling,
		groupDeveloperId: v1.DefaultProjectDeveloper,
	}); err != nil {
		log.Fatal().Err(err).Msg("Failed to init Project 1")
	}

	if err := pkg.CreateRelationProject(ctx, organizationId, projectTest2, map[string][]string{
		groupOwnerId:     v1.DefaultProjectOwner,
		groupAdminId:     v1.DefaultProjectAdmin,
		groupBillingId:   v1.DefaultProjectBilling,
		groupDeveloperId: v1.DefaultProjectDeveloper,
	}); err != nil {
		log.Fatal().Err(err).Msg("Failed to init Project 2")
	}

	// Add users to group
	if err := pkg.CreateOrUpdateRelationUserGroup(ctx, organizationId, user1, []string{groupAdminId}); err != nil {
		log.Fatal().Err(err).Msg("Failed to add user1 to admin group")
	}
	if err := pkg.CreateOrUpdateRelationUserGroup(ctx, organizationId, user2, []string{groupAdminId}); err != nil {
		log.Fatal().Err(err).Msg("Failed to add user2 to admin group")
	}
	if err := pkg.CreateOrUpdateRelationUserGroup(ctx, organizationId, user3, []string{groupBillingId}); err != nil {
		log.Fatal().Err(err).Msg("Failed to add user3 to billing group")
	}

	checkOrganizationPermission(ctx, organizationId, user1, adminOrgaPermission)
	checkProjectPermission(ctx, organizationId, projectTest1, user1, adminProjectPermission)
	checkProjectPermission(ctx, organizationId, projectTest2, user1, adminProjectPermission)

	checkOrganizationPermission(ctx, organizationId, user2, adminOrgaPermission)
	checkProjectPermission(ctx, organizationId, projectTest1, user2, adminProjectPermission)
	checkProjectPermission(ctx, organizationId, projectTest2, user2, adminProjectPermission)

	checkOrganizationPermission(ctx, organizationId, user3, billingOrgaPermission)
	checkProjectPermission(ctx, organizationId, projectTest1, user3, billingProjectPermission)
	checkProjectPermission(ctx, organizationId, projectTest2, user3, billingProjectPermission)

	// Change group of user2
	if err := pkg.CreateOrUpdateRelationUserGroup(ctx, organizationId, user2, []string{groupDeveloperId}); err != nil {
		log.Fatal().Err(err).Msg("Failed to update user2 to Developer group")
	}

	checkOrganizationPermission(ctx, organizationId, user1, adminOrgaPermission)
	checkProjectPermission(ctx, organizationId, projectTest1, user1, adminProjectPermission)
	checkProjectPermission(ctx, organizationId, projectTest2, user1, adminProjectPermission)

	checkOrganizationPermission(ctx, organizationId, user2, developerOrgaPermission)
	checkProjectPermission(ctx, organizationId, projectTest1, user2, developerProjectPermission)
	checkProjectPermission(ctx, organizationId, projectTest2, user2, developerProjectPermission)

	checkOrganizationPermission(ctx, organizationId, user3, billingOrgaPermission)
	checkProjectPermission(ctx, organizationId, projectTest1, user3, billingProjectPermission)
	checkProjectPermission(ctx, organizationId, projectTest2, user3, billingProjectPermission)

	if err := pkg.UpdateRelationGroupPermissions(ctx, organizationId, pkg.Group{
		Id:             groupDeveloperId,
		OrgaId:         organizationId,
		ProjectIds:     []string{projectTest1},
		PermissionSets: v1.DefaultProjectDeveloper,
	}); err != nil {
		log.Fatal().Err(err).Msg("Failed update Developer Group")
	}

	checkProjectPermission(ctx, organizationId, projectTest1, user2, developerProjectPermission)
	checkProjectPermission(ctx, organizationId, projectTest2, user2, noneProjectPermission)

	fmt.Printf("Check list permission for users on organization\n")
	userPermission, _ := permission.ListUserPermission(ctx, organizationId, entity.Organization, organizationId, user1)
	log.Info().Strs("userPermission", userPermission).Msgf("User %s Permission for Organization", user1)
	userPermission, _ = permission.ListUserPermission(ctx, organizationId, entity.Organization, organizationId, user2)
	log.Info().Strs("userPermission", userPermission).Msgf("User %s Permission for Organization", user2)
	userPermission, _ = permission.ListUserPermission(ctx, organizationId, entity.Organization, organizationId, user3)
	log.Info().Strs("userPermission", userPermission).Msgf("User %s Permission for Organization", user3)

	fmt.Printf("Check list permission for users on project 1\n")
	userPermission, _ = permission.ListUserPermission(ctx, organizationId, entity.Project, projectTest1, user1)
	log.Info().Strs("userPermission", userPermission).Msgf("User %s Permission for Project 1", user1)
	userPermission, _ = permission.ListUserPermission(ctx, organizationId, entity.Project, projectTest1, user2)
	log.Info().Strs("userPermission", userPermission).Msgf("User %s Permission for Project 1", user2)
	userPermission, _ = permission.ListUserPermission(ctx, organizationId, entity.Project, projectTest1, user3)
	log.Info().Strs("userPermission", userPermission).Msgf("User %s Permission for Project 1", user3)

	fmt.Printf("Check list permission for users on project 2\n")
	userPermission, _ = permission.ListUserPermission(ctx, organizationId, entity.Project, projectTest2, user1)
	log.Info().Strs("userPermission", userPermission).Msgf("User %s Permission for Project 2", user1)
	userPermission, _ = permission.ListUserPermission(ctx, organizationId, entity.Project, projectTest2, user2)
	log.Info().Strs("userPermission", userPermission).Msgf("User %s Permission for Project 2", user2)
	userPermission, _ = permission.ListUserPermission(ctx, organizationId, entity.Project, projectTest2, user3)
	log.Info().Strs("userPermission", userPermission).Msgf("User %s Permission for Project 2", user3)

	if err := pkg.DeleteRelationProject(ctx, organizationId, projectTest2); err != nil {
		log.Fatal().Err(err).Msg("Failed to delete project 2")
	}

	fmt.Printf("Check list permission for users on project 2\n")
	userPermission, _ = permission.ListUserPermission(ctx, organizationId, entity.Project, projectTest2, user1)
	log.Info().Strs("userPermission", userPermission).Msgf("User %s Permission for Project 2", user1)
	userPermission, _ = permission.ListUserPermission(ctx, organizationId, entity.Project, projectTest2, user2)
	log.Info().Strs("userPermission", userPermission).Msgf("User %s Permission for Project 2", user2)
	userPermission, _ = permission.ListUserPermission(ctx, organizationId, entity.Project, projectTest2, user3)
	log.Info().Strs("userPermission", userPermission).Msgf("User %s Permission for Project 2", user3)
}

func checkOrganizationPermission(ctx context.Context, organizationId, userId string, want []bool) {
	fmt.Printf("Check organization permission for user %s\n", userId)
	// Check Orga Permission
	for i, orgaPermission := range orgaPermissions {
		result := permission.CanAccess(ctx, organizationId, orgaPermission, organizationId, userId)
		if result != want[i] {
			log.Error().Str("userId", userId).Msgf("Permission %s check failed : got %t - want %t", orgaPermission, result, want[i])
		}
	}
}

func checkProjectPermission(ctx context.Context, organizationId, projectId, userId string, want []bool) {
	fmt.Printf("Check project permission for user %s on project %s\n", userId, projectId)
	// Check Project Permission
	for i, projectPermission := range projectPermissions {
		result := permission.CanAccess(ctx, organizationId, projectPermission, projectId, userId)
		if result != want[i] {
			log.Error().Str("projectId", projectId).Str("userId", userId).Msgf("Permission %s check failed : got %t - want %t", projectPermission, result, want[i])
		}
	}
}
