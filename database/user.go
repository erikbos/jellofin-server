package database

import (
	"errors"

	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"

	"github.com/miquels/notflix-server/idhash"
)

type UserStorage struct {
	dbHandle *sqlx.DB
}

func NewUserStorage(d *sqlx.DB) *UserStorage {
	return &UserStorage{
		dbHandle: d,
	}
}

type User struct {
	ID       string
	Username string
	Password string
}

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidPassword = errors.New("invalid password")
)

// Validate checks if the user exists and the password is correct.
func (u *UserStorage) Validate(username, password string) (user *User, err error) {
	var data User
	sqlerr := u.dbHandle.Get(&data, "SELECT * FROM users WHERE username=? LIMIT 1", username)
	if sqlerr != nil {
		return nil, ErrUserNotFound
	}

	err = bcrypt.CompareHashAndPassword([]byte(data.Password), []byte(password))
	if err != nil {
		return nil, ErrInvalidPassword
	}
	return &data, nil
}

// Insert inserts a new user into the database.
func (u *UserStorage) Insert(username, password string) (user *User, err error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user = &User{
		ID:       idhash.IdHash(username),
		Username: username,
		Password: string(hashedPassword),
	}

	tx, _ := u.dbHandle.Beginx()
	defer tx.Rollback()

	_, err = tx.NamedExec(`INSERT INTO users (id, username, password) `+
		`VALUES (:id, :username, :password)`, user)
	if err != nil {
		return
	}
	tx.Commit()
	return
}

// GetById retrieves a user from the database by their ID.
func (u *UserStorage) GetByID(userID string) (user *User, err error) {
	var data User
	if err := u.dbHandle.Get(&data, "SELECT * FROM users WHERE id=? LIMIT 1", userID); err != nil {
		return nil, ErrUserNotFound
	}
	// No need to return hashed pw
	data.Password = ""
	return &data, nil
}
