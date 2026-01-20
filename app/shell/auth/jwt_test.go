package auth

import (
	"testing"
	"time"
)

func TestJWTManager_GenerateAndVerify(t *testing.T) {
	secretKey := "test-secret-key"
	expiration := 1 * time.Hour
	manager := NewJWTManager(secretKey, expiration)

	username := "testuser"
	token, err := manager.Generate(username)
	if err != nil {
		t.Fatalf("生成 token 失败: %v", err)
	}

	if token == "" {
		t.Fatal("生成的 token 为空")
	}

	claims, err := manager.Verify(token)
	if err != nil {
		t.Fatalf("验证 token 失败: %v", err)
	}

	if claims.Username != username {
		t.Errorf("用户名不匹配: got %v, want %v", claims.Username, username)
	}
}

func TestJWTManager_VerifyExpiredToken(t *testing.T) {
	secretKey := "test-secret-key"
	expiration := 1 * time.Millisecond
	manager := NewJWTManager(secretKey, expiration)

	username := "testuser"
	token, err := manager.Generate(username)
	if err != nil {
		t.Fatalf("生成 token 失败: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	_, err = manager.Verify(token)
	if err == nil {
		t.Error("过期 token 应该验证失败")
	}
}

func TestJWTManager_VerifyInvalidToken(t *testing.T) {
	manager := NewJWTManager("test-secret-key", 1*time.Hour)

	tests := []struct {
		name  string
		token string
	}{
		{"空 token", ""},
		{"无效格式", "invalid.token.format"},
		{"伪造 token", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VybmFtZSI6ImZha2UifQ.fake"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := manager.Verify(tt.token)
			if err == nil {
				t.Error("无效 token 应该验证失败")
			}
		})
	}
}
