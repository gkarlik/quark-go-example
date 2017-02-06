package model

import (
	"fmt"
	"github.com/gkarlik/quark-go/data/access/rdbms"
	"github.com/gkarlik/quark-go/data/access/rdbms/gorm"
)

type User struct {
	ID       uint `gorm:"primary_key"`
	Login    string
	Password string
}

type UserRepository struct {
	*gorm.RepositoryBase
}

func NewUserRepository(c rdbms.DbContext) *UserRepository {
	repo := &UserRepository{
		RepositoryBase: &gorm.RepositoryBase{},
	}

	repo.SetContext(c)

	return repo
}

func (ur *UserRepository) FindByLogin(login string) (*User, error) {
	if login == "" {
		return nil, fmt.Errorf("Invalid username or password")
	}

	var user User
	if err := ur.First(&user, User{Login: login}); err != nil {
		return nil, err
	}
	return &user, nil
}
