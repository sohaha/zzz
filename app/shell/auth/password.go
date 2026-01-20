package auth

import (
	"fmt"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

type PasswordProvider struct {
	users map[string]string
	mu    sync.RWMutex
}

func NewPasswordProvider() *PasswordProvider {
	return &PasswordProvider{
		users: make(map[string]string),
	}
}

func (p *PasswordProvider) AddUser(username, password string) error {
	if username == "" {
		return fmt.Errorf("username cannot be empty")
	}
	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	p.users[username] = string(hashedPassword)
	return nil
}

func (p *PasswordProvider) Authenticate(username, password string) (bool, error) {
	p.mu.RLock()
	hashedPassword, exists := p.users[username]
	p.mu.RUnlock()

	// 使用假哈希防止时序攻击,确保无论用户名是否存在都执行 bcrypt
	dummyHash := "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
	hash := dummyHash
	if exists {
		hash = hashedPassword
	}

	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if !exists || err != nil {
		return false, nil
	}

	return true, nil
}
