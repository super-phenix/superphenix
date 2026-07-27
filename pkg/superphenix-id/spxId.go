package superphenixId

import (
	"fmt"

	"github.com/google/uuid"
)

// Metadata is an embeddable struct used to enforce the SPX ID framework
// defined in SPX-RFC0001 and the resource labels defined in SPX-RFC0002
type Metadata struct {
	OrgId               string `json:"orgID"`
	ProjectId           string `json:"projectID,omitempty"`
	ResourceEffectiveId string `json:"resourceEffectiveID,omitempty"`
	ResourceLocalId     string `json:"resourceLocalID,omitempty"`
}

func (m *Metadata) ConvertToSpxMetadata(info Metadata) error {
	// Add prefix and normalize name
	m.OrgId = info.OrgId
	m.ProjectId = info.ProjectId
	m.ResourceLocalId = info.ResourceLocalId

	// Check the information we've received respects the RFCs
	if err := m.check(); err != nil {
		return err
	}

	// We can safely compute the effective ID if necessary now
	return m.computeEffectiveID()
}

// GenerateFromProject generates the ID framework of a resource, with partial information from the parent project
func (m *Metadata) GenerateFromProject(project Metadata, localID string) error {
	m.OrgId = project.OrgId
	m.ProjectId = project.ProjectId
	m.ResourceLocalId = localID

	// Check the information we've received respects the RFCs
	if err := m.check(); err != nil {
		return err
	}

	// We can safely compute the effective ID if necessary now
	return m.computeEffectiveID()
}

// GenerateMetadata generates the ID framework of a resource, with given information
func (m *Metadata) GenerateMetadata(projectId, orgId, localID string) error {
	m.OrgId = orgId
	m.ProjectId = projectId
	m.ResourceLocalId = localID

	// Check the information we've received respects the RFCs
	if err := m.check(); err != nil {
		return err
	}

	// We can safely compute the effective ID if necessary now
	return m.computeEffectiveID()
}

// GetOrgID returns the SPXID of the parent organization
func (m *Metadata) GetOrgID() string {
	return ToSPXID(m.OrgId)
}

// GetProjectID returns the SPXID of the parent project
func (m *Metadata) GetProjectID() string {
	return ToSPXID(m.ProjectId)
}

// GetResourceEffectiveID returns the SPXID of the current resource
func (m *Metadata) GetResourceEffectiveID() string {
	return ToSPXID(m.ResourceEffectiveId)
}

// GetResourceLocalID returns the local ID of the resource
func (m *Metadata) GetResourceLocalID() string {
	return m.ResourceLocalId
}

// GetLabels returns the K8S labels enforcing SPX-RFC0002 for the given resource.
func (m *Metadata) GetLabels() map[string]string {
	labels := make(map[string]string)

	// Every resource belongs to an organization
	labels[SpxLabelOrganizationID] = m.GetOrgID()

	// This structure identifies a project or a resource within a project
	if m.GetProjectID() != "" {
		labels[SpxLabelProjectID] = m.GetProjectID()
	}

	// This structure identifies a resource within a project
	if m.GetResourceLocalID() != "" {
		labels[SpxLabelResourceLocalID] = m.GetResourceLocalID()
		labels[SpxLabelResourceEffectiveID] = m.GetResourceEffectiveID()
	}

	return labels
}

// ToSPXID converts an UUIDv4/UUIDv5 to its presentable SPXID form as defined in SPX-RFC0001
// Format : {prefix}-{id}
func ToSPXID(id string) string {
	// If the ID doesn't exist for this resource, do not try to present it as an SPXID
	if id == "" {
		return ""
	}

	return fmt.Sprintf("%s-%s", FrameworkPrefix(), id)
}

// computeEffectiveID creates the effective ID of the resource using its local ID/project ID
func (m *Metadata) computeEffectiveID() error {
	// Only compute if we're representing a resource
	if m.ProjectId == "" || m.ResourceLocalId == "" {
		return InvalidProjectOrResourceLocalID
	}

	ns, err := uuid.Parse(m.ProjectId)
	if err != nil {
		// This should never happen, `check` should have been called right before
		return InvalidProjectID
	}

	m.ResourceEffectiveId = uuid.NewSHA1(ns, []byte(m.ResourceLocalId)).String()
	return nil
}

// check verifies that the structure is valid
func (m *Metadata) check() error {
	// Every resource at least belongs to an organization
	if m.OrgId == "" {
		return OrgIDEmpty
	}

	// If this structure identifies a resource, it must belong to a project
	if m.ResourceLocalId != "" && m.ProjectId == "" {
		return ProjectIDEmpty
	}

	// Check that the UUIDs passed to generate the struct are valid
	if err := m.checkUUIDs(); err != nil {
		return err
	}

	return nil
}

// checkUUIDs verifies that the IDs contained within the structure are valid
func (m *Metadata) checkUUIDs() error {
	// The structure must always have an organization ID
	if !isValidUUID(m.OrgId) {
		return InvalidOrgID
	}

	// There might be no project ID (if this structure defines an organization for example)
	if m.ProjectId != "" && !isValidUUID(m.ProjectId) {
		return InvalidProjectID
	}

	// If this structure identifies a resource, the local ID must be valid
	if m.ResourceLocalId != "" && !isValidName(m.ResourceLocalId) {
		return InvalidResourceLocalID
	}

	return nil
}

// isValidUUID returns whether a string represents a valid UUID
func isValidUUID(uuidString string) bool {
	_, err := uuid.Parse(uuidString)
	if err != nil {
		return false
	}

	return true
}

// isValidName enforces SPX-RFC0001 on names associated with IDs
func isValidName(name string) bool {
	if name == "" || len(name) > nameMaxLength {
		return false
	}

	return nameEnforcer.MatchString(name)
}
