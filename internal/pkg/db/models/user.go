package models

// User is an account. The first user created becomes the admin; further users
// are admin-provisioned (no self-registration). PasswordHash is null-able so a
// passwordless single-user install can exist without one.
type User struct {
	Base

	Username     string
	PasswordHash *string
	IsAdmin      bool
}

// TableName pins the table name.
func (User) TableName() string {
	return "users"
}
