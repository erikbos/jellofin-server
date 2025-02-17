package main

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

// func init() {
// 	Users.db = []User{
// 		{ID: "1", Username: "1234", Password: "1234"},
// 		{ID: "2", Username: "erik", Password: "erik"},
// 	}
// }

// var Users UserRepo

// type UserRepo struct {
// 	mu sync.Mutex
// 	db []User
// }

// type User struct {
// 	ID       string
// 	Username string
// 	Password string
// }

// // Lookup user by name
// func (u *UserRepo) Lookup(user string) *User {
// 	Users.mu.Lock()
// 	defer Users.mu.Unlock()

// 	for _, u := range u.db {
// 		if u.Username == user {
// 			return &u
// 		}
// 	}
// 	return nil
// }

// // Validate user by username & password
// func (u *UserRepo) Validate(username, password string) *User {
// 	Users.mu.Lock()
// 	defer Users.mu.Unlock()

// 	for _, u := range u.db {
// 		if u.Username == username && u.Password == password {
// 			return &u
// 		}
// 	}
// 	return nil
// }

////////////////////////////////////////////////////////////////////////////////
//
// in-memory access token store for now

func init() {
	AccessTokens.db = make(map[string]*AccessToken)
}

var AccessTokens AccessTokenRepo

type AccessTokenRepo struct {
	mu sync.Mutex
	db map[string]*AccessToken
}

type AccessToken struct {
	accesstoken string
	session     *JFSessionInfo
}

// New generates new token
func (s *AccessTokenRepo) New(session *JFSessionInfo) string {
	AccessTokens.mu.Lock()
	defer AccessTokens.mu.Unlock()

	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		panic(err)
	}
	token := hex.EncodeToString(bytes)

	at := &AccessToken{
		accesstoken: token,
		session:     session,
	}
	s.db[token] = at

	return token
}

// Lookup accesstoken details
func (s *AccessTokenRepo) Lookup(id string) *AccessToken {
	AccessTokens.mu.Lock()
	defer AccessTokens.mu.Unlock()

	if at, ok := s.db[id]; ok {
		return at
	}
	return nil
}
