package models

type User struct {
	Email        string `ch:"email"`
	PasswordHash string `ch:"password_hash"`
	Username     string `ch:"username"`
	Active       bool   `ch:"active"`
	AccessRights string `ch:"access_rights"`
}

type Token struct {
	Token     string `ch:"token"`
	Email     string `ch:"email"`
	CreatedAt string `ch:"created_at"`
	ExpiresAt string `ch:"expires_at"`
}
