package provider

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/iam/organization"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/services/project/project"

	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db/model"

	kratos "github.com/ory/kratos-client-go"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm/clause"
)

type Traits struct {
	Email string `json:"email"`
	Name  struct {
		First string `json:"first"`
		Last  string `json:"last"`
	} `json:"name"`
}

// InitializeKratosUser creates or updates a DB User from a Kratos User.
// Matches by email; creates if new (and initialize a default org and project), updates name/email if existing.
func InitializeKratosUser(r *http.Request, session *kratos.Session) *http.Request {

	var traits Traits
	tmp, err := json.Marshal(session.Identity.Traits)
	if err == nil {
		err = json.Unmarshal(tmp, &traits)
	}
	if err != nil {
		log.Error().Ctx(r.Context()).Any("traits", session.Identity.Traits).Msg("Couldn't cast traits")
		return r
	}

	newUser := model.User{
		Firstname:  &traits.Name.First,
		Lastname:   &traits.Name.Last,
		Email:      traits.Email,
		Provider:   db.KratosProvider,
		ProviderId: session.Identity.Id,
		IsActive:   config.Global.UserSettings.UserIsActiveOnCreate,
		PersonalOrg: []model.Organization{{
			Name: "Personal Org",
		}},
		GuestOrg: nil,
	}

	result := db.Client.Where(model.User{
		Provider:   db.KratosProvider,
		ProviderId: session.Identity.Id,
	}).Preload(clause.Associations).FirstOrCreate(&newUser)

	if result.RowsAffected == 1 { // Record create, permission initialization
		log.Info().Str("userId", newUser.ID.String()).Strs("provider", []string{db.KratosProvider, session.Identity.Id}).Msg("Successfully created new user, starting permission initialization")
		// TODO: handle errors during initialization
		// Init organization in Permify (and default groups)
		if err := organization.InitOrganization(r.Context(), newUser.PersonalOrg[0], newUser.ID); err == nil {
			// Init default project (DB and Permify)
			_, _ = project.InitializeProject(r.Context(), newUser.ID, newUser.PersonalOrg[0].ID, "default")
		}
	} else { //	Already existing user, update it
		log.Info().Ctx(r.Context()).Str("userId", newUser.ID.String()).Strs("provider", []string{db.KratosProvider, session.Identity.Id}).Msg("Already existing user, updating it")
		newUser.Firstname = &traits.Name.First
		newUser.Lastname = &traits.Name.Last
		newUser.Email = traits.Email
		// Omit association, just update the user
		db.Client.Omit(clause.Associations).Save(&newUser)
	}

	// Add userId to context
	r = r.WithContext(context.WithValue(r.Context(), consts.ContextUserId, newUser.ID.String()))
	return r
}
