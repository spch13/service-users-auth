package repository

import (
	"fmt"
	"github.com/spch13/service-users-auth/internal/model"
	"sync"
)

type UserRepositoryInMem struct {
	mu    sync.RWMutex
	users map[string]*model.User
}

// NewUserRepoInMem returns a new in-memory user store
func NewUserRepoInMem() *UserRepositoryInMem {
	return &UserRepositoryInMem{
		users: make(map[string]*model.User),
	}
}

func (r *UserRepositoryInMem) Save(user *model.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.users[user.Username] != nil {
		return fmt.Errorf("record %s already exists", user.Username)
	}

	r.users[user.Username] = user.Clone()
	return nil
}

// Find finds a user by username
func (r *UserRepositoryInMem) Find(username string) (*model.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.users[username]
	if !ok {
		return nil, ErrNotFound
	}

	return user.Clone(), nil
}
