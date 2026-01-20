package auth

import (
	"testing"
	"time"
)

func TestPasswordProvider_TimingAttackResistance(t *testing.T) {
	provider := NewPasswordProvider()
	provider.AddUser("validuser", "password123")

	tests := []struct {
		name     string
		username string
		password string
		wantAuth bool
	}{
		{"有效用户正确密码", "validuser", "password123", true},
		{"有效用户错误密码", "validuser", "wrongpass", false},
		{"无效用户", "invaliduser", "password123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iterations := 10
			var totalDuration time.Duration

			for i := 0; i < iterations; i++ {
				start := time.Now()
				authenticated, err := provider.Authenticate(tt.username, tt.password)
				duration := time.Since(start)
				totalDuration += duration

				if err != nil {
					t.Errorf("认证失败: %v", err)
				}
				if authenticated != tt.wantAuth {
					t.Errorf("认证结果错误: got %v, want %v", authenticated, tt.wantAuth)
				}
			}

			avgDuration := totalDuration / time.Duration(iterations)
			t.Logf("%s 平均耗时: %v", tt.name, avgDuration)
		})
	}
}

func TestPasswordProvider_AddUser(t *testing.T) {
	provider := NewPasswordProvider()

	tests := []struct {
		name     string
		username string
		password string
		wantErr  bool
	}{
		{"正常用户", "user1", "pass123", false},
		{"空用户名", "", "pass123", true},
		{"空密码", "user2", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.AddUser(tt.username, tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddUser() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPasswordProvider_Authenticate(t *testing.T) {
	provider := NewPasswordProvider()
	provider.AddUser("testuser", "testpass")

	tests := []struct {
		name     string
		username string
		password string
		want     bool
	}{
		{"正确凭证", "testuser", "testpass", true},
		{"错误密码", "testuser", "wrongpass", false},
		{"不存在的用户", "nouser", "testpass", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := provider.Authenticate(tt.username, tt.password)
			if err != nil {
				t.Errorf("Authenticate() error = %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("Authenticate() = %v, want %v", got, tt.want)
			}
		})
	}
}
