package data

import (
	"database/sql"
	"errors"
)

var (
	ErrRecordNotFound = errors.New("record not found")
	ErrEditConflict   = errors.New("edit conflict")
)

type Models struct {
	Book        BookModel
	Tokens      TokenModel // Add a new Tokens field.
	Users       UserModel
	Permissions PermissionModel
}

func NewModels(db *sql.DB) Models {
	return Models{
		Book:        BookModel{DB: db},
		Tokens:      TokenModel{DB: db}, // Initialize a new TokenModel instance.
		Users:       UserModel{DB: db},
		Permissions: PermissionModel{DB: db},
	}
}
