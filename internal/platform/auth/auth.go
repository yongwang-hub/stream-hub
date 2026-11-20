// Package auth 认证与鉴权：基于 golang-jwt/jwt/v5 的 HS256 签发/验证，以及路由关卡中间件。
//
// 这是**平台级基础设施**：只负责"你是谁、你能不能"，不含任何账号管理 CRUD。
// 账号/角色的存储与后台管理在 internal/identity 业务切片里。
//
// 门面原则：本包对外只暴露 Issue/Verify/Middleware 等，库类型 jwt.Token 不外泄。
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// 对外错误（中间件据此给用户回不同 msg）。
var (
	ErrTokenInvalid = errors.New("token invalid")
	ErrTokenExpired = errors.New("token expired")
)

// Claims 自定义声明：内嵌标准 RegisteredClaims（sub/iat/exp/iss/aud/jti）。
// token 只回答"你是谁"(Subject=userID)，绝不放角色/权限——"能不能"每次实时查库。
// MustChange 是唯一例外，只为让首登强制改密关卡少查一次库。
type Claims struct {
	MustChange bool `json:"mcp,omitempty"`
	jwt.RegisteredClaims
}

// Authenticator 持有密钥、签发参数，以及账号存储依赖（登录/鉴权要用）。
type Authenticator struct {
	secret   []byte
	ttl      time.Duration
	parser   *jwt.Parser
	accounts AccountStore // 可为 nil：纯 JWT 单测场景
}

// New 创建验证器。WithValidMethods 白名单锁 HS256，防 alg=none 算法混淆。
func New(secret string, ttl time.Duration, accounts AccountStore) *Authenticator {
	return &Authenticator{
		secret:   []byte(secret),
		ttl:      ttl,
		accounts: accounts,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithLeeway(5*time.Second),
		),
	}
}

// Issue 签发 token：sub 传用户 ID，mustChange 标记首登是否需强制改密。
func (a *Authenticator) Issue(sub string, mustChange bool) (string, error) {
	now := time.Now()
	claims := Claims{
		MustChange: mustChange,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   sub,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(a.ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.secret)
}

// Verify 校验并解出 claims，任何库错误归一成 ErrTokenExpired / ErrTokenInvalid。
func (a *Authenticator) Verify(token string) (*Claims, error) {
	claims := &Claims{}
	_, err := a.parser.ParseWithClaims(token, claims, a.keyFunc)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}
	return claims, nil
}

func (a *Authenticator) keyFunc(t *jwt.Token) (any, error) {
	if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, ErrTokenInvalid
	}
	return a.secret, nil
}

// TokenTTL 返回签发时长，供登录/改密接口告诉客户端过期秒数。
func (a *Authenticator) TokenTTL() time.Duration { return a.ttl }

// ---- context 传递 ----

type claimsKey struct{}

func WithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, c)
}

// FromContext 取本请求的 claims；未认证返回 nil。
func FromContext(ctx context.Context) *Claims {
	c, _ := ctx.Value(claimsKey{}).(*Claims)
	return c
}
