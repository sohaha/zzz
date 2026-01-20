package shell

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/sohaha/zlsgo/zlog"
	"github.com/sohaha/zlsgo/zstring"
	"github.com/sohaha/zzz/app/shell/auth"
	"github.com/sohaha/zzz/app/shell/handler"
)

type Config struct {
	Port     string
	Host     string
	Username string
	Password string
	Shell    string
}

func Start(cfg Config) error {
	if cfg.Username == "" {
		cfg.Username = "admin"
	}

	if cfg.Password == "" {
		cfg.Password = zstring.Rand(12)
		fmt.Fprintf(os.Stderr, "\n⚠️  随机密码已生成: %s\n\n", cfg.Password)
		zlog.Info("随机密码已生成,请查看控制台输出")
	}

	authProvider := auth.NewPasswordProvider()
	if err := authProvider.AddUser(cfg.Username, cfg.Password); err != nil {
		return fmt.Errorf("failed to add user: %w", err)
	}

	secretKey := zstring.Rand(32)
	jwtManager := auth.NewJWTManager(secretKey, 24*time.Hour)

	h := handler.NewHandler(authProvider, jwtManager, cfg.Shell)

	addr := fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)
	zlog.Infof("启动 Web Terminal 服务器")
	zlog.Infof("监听地址: http://%s", addr)
	zlog.Infof("用户名: %s", cfg.Username)

	return http.ListenAndServe(addr, h)
}
