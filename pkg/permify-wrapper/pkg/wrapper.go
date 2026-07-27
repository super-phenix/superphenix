package pkg

import (
	"context"
	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permissionSet"

	"github.com/rs/zerolog/log"

	utils "github.com/super-phenix/superphenix/pkg/permify-wrapper/internal"

	v1 "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1"
	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/entity"
	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/bundle"
	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/data"
	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/schema"
	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/tenancy"
)

type Organization struct {
	Id   string
	Name string
}

// MapGroupPermissionSets - map which associate a groupId with a list of PermissionSet
type MapGroupPermissionSets = map[string][]string

type Group struct {
	Id             string
	OrgaId         string
	ProjectIds     []string
	PermissionSets []string
}

// CreateRelationOrganization initialize a organization in Permify with the given permissionSet for each group
func CreateRelationOrganization(ctx context.Context, orga Organization, groups MapGroupPermissionSets) error {

	if err := tenancy.Create(ctx, orga.Id, orga.Name); err != nil {
		return err
	}

	if err := schema.Write(ctx, orga.Id, v1.DefaultSchema); err != nil {
		return err
	}

	if err := bundle.Write(ctx, orga.Id, []*bundle.DataBundle{bundle.CreateProjectBundle}); err != nil {
		return err
	}

	for id, permissionSets := range groups {
		if err := UpdateRelationOrganizationGroup(ctx, orga.Id, id, permissionSets); err != nil {
			log.Error().Err(err).Any("orga", orga).Str("groupId", id).Strs("permissionSets", permissionSets).Msg("Failed to update group in organization")
		}
	}

	return nil
}

// CreateRelationProject initialize a project in Permify with the given permissionSet for each group
func CreateRelationProject(ctx context.Context, orgaId, projectId string, groups MapGroupPermissionSets) error {
	if err := bundle.Run(ctx, orgaId, bundle.CreateProjectBundle.Name, map[string]string{"projectId": projectId, "orgaId": orgaId}); err != nil {
		return err
	}

	for id, permissionSets := range groups {
		if len(permissionSets) > 0 {
			if err := UpdateRelationProjectGroup(ctx, orgaId, projectId, id, permissionSets); err != nil {
				log.Error().Err(err).Str("orgaId", orgaId).Str("projectId", projectId).Str("groupId", id).Strs("permissionSets", permissionSets).Msg("Failed to update group in project")
			}
		}
	}

	return nil
}

func UpdateRelationOrganizationGroup(ctx context.Context, orgaId, groupId string, permissionSets []string) error {
	// Clean old
	if err := data.Delete(ctx, orgaId, &v1.TupleFilter{
		Entity: &v1.EntityFilter{
			Type: entity.Organization,
			Ids:  []string{orgaId},
		},
		Subject: &v1.SubjectFilter{
			Type:     entity.Group,
			Ids:      []string{groupId},
			Relation: "assignee",
		},
	}); err != nil {
		return err
	}

	var tuples []*v1.Tuple
	for _, ps := range permissionSets {
		// Check if the permission is for an organization
		if permissionSet.PermissionSetsEntityMap[ps] == entity.Organization {
			tuples = append(tuples, utils.GetGroupTuple(ps, orgaId, groupId))
		}
	}

	if err := data.Write(ctx, orgaId, tuples); err != nil {
		return err
	}

	return nil
}

func UpdateRelationProjectGroup(ctx context.Context, orgaId, projectId, groupId string, permissionSets []string) error {
	// Clean old
	if err := data.Delete(ctx, orgaId, &v1.TupleFilter{
		Entity: &v1.EntityFilter{
			Type: entity.Project,
			Ids:  []string{projectId},
		},
		Subject: &v1.SubjectFilter{
			Type:     entity.Group,
			Ids:      []string{groupId},
			Relation: "assignee",
		},
	}); err != nil {
		return err
	}

	var tuples []*v1.Tuple
	for _, ps := range permissionSets {
		// Check if the permission is for a project
		if permissionSet.PermissionSetsEntityMap[ps] == entity.Project {
			tuples = append(tuples, utils.GetGroupTuple(ps, projectId, groupId))
		}
	}
	if len(tuples) > 0 {
		if err := data.Write(ctx, orgaId, tuples); err != nil {
			return err
		}
	}

	return nil
}

func CreateOrUpdateRelationUserGroup(ctx context.Context, orgaId, userId string, groupId []string) error {

	if err := DeleteRelationUser(ctx, orgaId, userId); err != nil {
		return err
	}

	for _, gId := range groupId {
		if err := data.Write(ctx, orgaId, []*v1.Tuple{
			{
				Entity: &v1.Entity{
					Type: entity.Group,
					Id:   gId,
				},
				Relation: "assignee",
				Subject: &v1.Subject{
					Type:     entity.User,
					Id:       userId,
					Relation: "",
				},
			}}); err != nil {
			return err
		}
	}

	return nil
}

func DeleteRelationOrganization(ctx context.Context, orgaId string) error {
	return tenancy.Delete(ctx, orgaId)
}

func DeleteRelationProject(ctx context.Context, orgaId, projectId string) error {
	if err := data.Delete(ctx, orgaId, &v1.TupleFilter{
		Entity: &v1.EntityFilter{
			Type: entity.Project,
			Ids:  []string{projectId},
		},
	}); err != nil {
		return err
	}

	return nil
}

func DeleteRelationUser(ctx context.Context, orgaId, userId string) error {
	if err := data.Delete(ctx, orgaId, &v1.TupleFilter{
		Subject: &v1.SubjectFilter{Type: entity.User, Ids: []string{userId}},
	}); err != nil {
		return err
	}
	return nil
}

// UpdateRelationGroupPermissions Update Group permissionSet for every entity
func UpdateRelationGroupPermissions(ctx context.Context, orgaId string, group Group) error {
	// Clean old
	if err := data.Delete(ctx, orgaId, &v1.TupleFilter{
		Subject: &v1.SubjectFilter{
			Type:     entity.Group,
			Ids:      []string{group.Id},
			Relation: "assignee",
		},
	}); err != nil {
		return err
	}

	var tuples []*v1.Tuple
	for _, ps := range group.PermissionSets {
		switch permissionSet.PermissionSetsEntityMap[ps] {
		case entity.Organization:
			tuples = append(tuples, utils.GetGroupTuple(ps, group.OrgaId, group.Id))
			break
		case entity.Project:
			for _, id := range group.ProjectIds {
				tuples = append(tuples, utils.GetGroupTuple(ps, id, group.Id))
			}
			break
		}
	}

	if err := data.Write(ctx, orgaId, tuples); err != nil {
		return err
	}

	return nil
}
