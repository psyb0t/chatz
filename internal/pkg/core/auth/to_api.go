package auth

import (
	"github.com/psyb0t/chatz/internal/pkg/db/models"
	api "github.com/psyb0t/chatz/internal/pkg/http/api"
)

// UserToAPI projects a stored user row to the wire shape. PasswordHash is never
// exposed.
func UserToAPI(u *models.User) api.User {
	return api.User{
		Id:        u.ID,
		Username:  u.Username,
		IsAdmin:   u.IsAdmin,
		CreatedAt: u.CreatedAt,
	}
}
