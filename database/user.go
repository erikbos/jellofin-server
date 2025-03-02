package database

import (
	"errors"

	"golang.org/x/crypto/bcrypt"

	"github.com/miquels/notflix-server/idhash"
)

type User struct {
	Id       string
	Username string
	Password string
}

var (
	ErrUserNotFound    = errors.New("user not found")
	ErrInvalidPassword = errors.New("invalid password")
)

// dbUserValidate checks if the user exists and the password is correct.
func (d *DatabaseRepo) UserValidate(username, password *string) (user *User, err error) {
	var data User
	sqlerr := d.dbHandle.Get(&data, "SELECT * FROM users WHERE username=? LIMIT 1", username)
	if sqlerr != nil {
		return nil, ErrUserNotFound

	}
	err = bcrypt.CompareHashAndPassword([]byte(data.Password), []byte(*password))
	if err != nil {
		return nil, ErrInvalidPassword
	}
	return &data, nil
}

// UserInsert inserts a new user into the database.
func (d *DatabaseRepo) UserInsert(username, password string) (user *User, err error) {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		return nil, err
	}

	user = &User{
		Id:       idhash.IdHash(username),
		Username: username,
		Password: string(hashedPassword),
	}

	tx, _ := d.dbHandle.Beginx()
	_, err = tx.NamedExec(`INSERT INTO users (id, username, password) `+
		`VALUES (:id, :username, :password)`, user)
	if err != nil {
		tx.Rollback()
	}
	tx.Commit()
	return
}

// UserGetById retrieves a user from the database by their ID.
func (d *DatabaseRepo) UserGetById(id string) (user *User, err error) {
	var data User
	sqlerr := d.dbHandle.Get(&data, "SELECT * FROM users WHERE id=? LIMIT 1", id)
	if sqlerr != nil {
		return nil, ErrUserNotFound
	}
	data.Password = ""
	return &data, nil
}
