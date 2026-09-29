// Command tokengen 签发平台自签访问令牌，用于本地联调与运维排障。
//
// 用法：
//
//	go run ./cmd/tokengen -sub user:alice -tid tenant-a
//	go run ./cmd/tokengen -sub agent:writer -tid tenant-a -roles agent,editor -ttl 2h
//
// 签名密钥从环境变量 JWT_SECRET 读取（未设置时回退 ENCRYPTION_KEY），也可用 -secret 显式传入。
// 注意：本命令会向标准输出打印令牌本身，属于凭据，请勿粘贴到聊天、工单或日志系统。
//
// 退出码：0 成功；2 参数或密钥错误。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	platformauth "InkLore/internal/platform/auth"
	"InkLore/internal/platform/timeutil"
)

func main() {
	subject := flag.String("sub", "", "主体标识，例如 user:alice")
	tenantID := flag.String("tid", "", "租户标识，例如 tenant-a")
	roles := flag.String("roles", "", "角色列表，逗号分隔")
	userID := flag.Int64("uid", 0, "数字用户 ID（可选）")
	ttl := flag.Duration("ttl", platformauth.DefaultTokenTTL, "有效期，例如 2h")
	secret := flag.String("secret", "", "签名密钥（默认读 JWT_SECRET / ENCRYPTION_KEY）")
	jsonOutput := flag.Bool("json", false, "同时输出令牌声明（便于核对，不含密钥）")
	flag.Parse()

	if strings.TrimSpace(*subject) == "" {
		_, _ = fmt.Fprintln(os.Stderr, "tokengen: -sub 必填")
		os.Exit(2)
	}

	signingKey := strings.TrimSpace(*secret)
	if signingKey == "" {
		signingKey = strings.TrimSpace(os.Getenv("JWT_SECRET"))
	}
	if signingKey == "" {
		signingKey = strings.TrimSpace(os.Getenv("ENCRYPTION_KEY"))
	}
	if signingKey == "" {
		_, _ = fmt.Fprintln(os.Stderr, "tokengen: 未找到签名密钥，请设置 JWT_SECRET（或 ENCRYPTION_KEY），或使用 -secret")
		os.Exit(2)
	}

	now := timeutil.Now().Unix()
	claims := platformauth.Claims{
		Subject:   strings.TrimSpace(*subject),
		TenantID:  strings.TrimSpace(*tenantID),
		UserID:    *userID,
		Issuer:    platformauth.TokenIssuer,
		IssuedAt:  now,
		ExpiresAt: now + int64((*ttl)/time.Second),
	}
	for _, role := range strings.Split(*roles, ",") {
		if trimmed := strings.TrimSpace(role); trimmed != "" {
			claims.Roles = append(claims.Roles, trimmed)
		}
	}

	token, err := platformauth.SignToken(claims, signingKey)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "tokengen: 签发失败: %v\n", err)
		os.Exit(2)
	}

	if *jsonOutput {
		payload, marshalErr := json.Marshal(claims)
		if marshalErr != nil {
			_, _ = fmt.Fprintf(os.Stderr, "tokengen: 序列化声明失败: %v\n", marshalErr)
			os.Exit(2)
		}
		fmt.Println(string(payload))
	}
	fmt.Println(token)
}
