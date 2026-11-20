package dbaas

import (
	spxId "github.com/super-phenix/superphenix/pkg/superphenix-id"

	"github.com/google/uuid"
)

// EipEffectiveID returns the effective ID of the EIP the sfs-dbaas chart renders for a database.
func EipEffectiveID(projectId, localId string) (string, error) {
	return effectiveID(projectId, eipLocalIDPrefix+localId)
}

// effectiveID computes a resource effective ID like the uuidv5 Argo plugin.
func effectiveID(projectId, localId string) (string, error) {
	ns, err := uuid.Parse(projectId)
	if err != nil {
		return "", err
	}
	return spxId.ToSPXID(uuid.NewSHA1(ns, []byte(localId)).String()), nil
}
