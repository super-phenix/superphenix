package utils

import (
	v1 "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1"
	"github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/entity"
	basePermission "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permission"
	basePSet "github.com/super-phenix/superphenix/pkg/permify-wrapper/pkg/base/v1/permissionSet"
)

// StrToTuple convert string to Permify Tuple
// from : document:1#editor@user:1
// to :
//
//	{
//		Entity: &v1.Entity{
//			Type: "document",
//			Id:   "1",
//		},
//		Relation: "editor",
//		Subject:  &v1.Subject{
//			Type: "user",
//			Id:   "1",
//			Relation: "",
//		},
//	}

// GetTuple convert permission to a Permify Tuple
func GetTuple(permission, entityId, subjectType, subjectId string) *v1.Tuple {
	entityType := basePermission.PermissionsEntityMap[permission]

	attr := &v1.Tuple{}
	attr.Entity = &v1.Entity{
		Type: entityType,
		Id:   entityId,
	}

	attr.Relation = permission

	attr.Subject = &v1.Subject{
		Type:     subjectType,
		Id:       subjectId,
		Relation: "",
	}
	return attr
}

func GetGroupTuple(permissionSet, entityId, groupId string) *v1.Tuple {
	entityType := basePSet.PermissionSetsEntityMap[permissionSet]

	attr := &v1.Tuple{}
	attr.Entity = &v1.Entity{
		Type: entityType,
		Id:   entityId,
	}

	attr.Relation = permissionSet

	attr.Subject = &v1.Subject{
		Type:     entity.Group,
		Id:       groupId,
		Relation: "assignee",
	}
	return attr
}
