// Package auth 认证与鉴权：基于 golang-jwt/jwt/v5 的 HS256 签发/验证。
//
// 上一版手写 HS256 是为了祛魅——看清 JWT = header.payload.signature + HMAC。
// 这一版换成熟库，本质不变，但省掉了自己维护的坑：
//   - 算法白名单（拒绝 alg=none 与算法混淆攻击，库默认 + 我们显式双保险）
//   - exp/nbf/iat 时间校验与时钟偏移容忍（分布式部署刚需，WithLeeway）
//   - 密钥回调模式：解析时按 token 声明的算法决定"用什么密钥验"，防误用
//   - 未来升级 RS256/ES256/JWK 轮换，只在这一层加代码，业务零改动
//
// 门面原则：本包对外只暴露 Issue/Verify/Middleware/RequirePermission/Login，
// 库类型 jwt.Token/jwt.Claims 不外泄，业务层完全不知道里面用的是哪家库。
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// 对外错误（保持上一版语义，middleware 据此给用户回不同 msg）。
var (
	ErrTokenInvalid = errors.New("token invalid")
	ErrTokenExpired = errors.New("token expired")
)

// Claims 自定义声明：内嵌库的标准 RegisteredClaims（含 sub/iat/exp/iss/aud/jti）。
//
// 现代要点——token 只回答"你是谁"(Subject=userID)，绝不放角色/权限：
// "能不能"每次实时查库判定，于是禁用账号、调整权限、改密码都能立刻生效，
// 不会被已签发 token 的 TTL 卡住。MustChange 是唯一例外的一点授权元数据，
// 只为让"首登强制改密"关卡少查一次库（改密后重签即自动清除）。
type Claims struct {
	MustChange bool `json:"mcp,omitempty"` // must change password
	jwt.RegisteredClaims
}

// Authenticator 持有对称密钥、签发参数，以及对账号存储的依赖（登录/鉴权要用）。
type Authenticator struct {
	secret   []byte
	ttl      time.Duration
	parser   *jwt.Parser
	accounts AccountStore // 可为 nil：纯 JWT 场景（签发/验签/中间件签名校验）不需要它
}

// New 创建验证器。accounts 提供登录校验与权限判定所需的数据层；
// 若只做无库的 JWT 单测可传 nil。WithValidMethods 白名单锁 HS256：
// 库解析时如果 token 头部声明的算法不在这个列表，直接拒绝——这就是防 alg=none 的姿势。
func New(secret string, ttl time.Duration, accounts AccountStore) *Authenticator {
	return &Authenticator{
		secret:   []byte(secret),
		ttl:      ttl,
		accounts: accounts,
		parser: jwt.NewParser(
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithLeeway(5*time.Second), // 允许 ±5s 时钟偏差，避免服务器时间不齐导致误过期
		),
	}
}

// Issue 签发 token：sub 传用户 ID（字符串形式），mustChange 标记首登是否需强制改密。
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
	// NewWithClaims 声明用 HS256；SignedString 内部完成 base64url + HMAC。
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.secret)
}

// Verify 校验并解出 claims。任何库报错都归一成我们的两种错误，
// 上层不感知库错误类型——这是门面对"依赖泄漏"的隔断。
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

// keyFunc 是密钥回调：库要求"看 token 声明的算法决定用哪把钥匙"。
// 我们只有对称 HS256 一把钥，非 HMAC 类型一律拒绝——与 WithValidMethods 双保险。
func (a *Authenticator) keyFunc(t *jwt.Token) (any, error) {
	if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, ErrTokenInvalid
	}
	return a.secret, nil
}

// ---- context 传递：验证通过后把 Claims 挂到请求上下文，下游随便取 ----

type claimsKey struct{}

// WithClaims 把 claims 放进 ctx（中间件专用）。
func WithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, c)
}

// FromContext 取本请求的 claims；没认证过则返回 nil。
func FromContext(ctx context.Context) *Claims {
	c, _ := ctx.Value(claimsKey{}).(*Claims)
	return c
}
