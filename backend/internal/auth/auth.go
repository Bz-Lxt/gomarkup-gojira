package auth

import (
	"context"
	"net/http"
	"strings"
	"time"
	"unicode"

	"gojira/internal/domain"
	"gojira/internal/platform"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jmoiron/sqlx"
	"golang.org/x/crypto/bcrypt"
)

const (
	AccessTTL  = 2 * time.Hour
	RefreshTTL = 7 * 24 * time.Hour
)

type Service struct {
	DB     *sqlx.DB
	Secret []byte
}

type Claims struct {
	UserID   int64  `json:"uid"`
	Username string `json:"uname"`
	Role     string `json:"role"`
	Kind     string `json:"kind"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken  string                  `json:"access_token"`
	RefreshToken string                  `json:"refresh_token"`
	ExpiresIn    int64                   `json:"expires_in"`
	User         *platform.UserPrincipal `json:"user"`
}

type loginBody struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type registerBody struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type refreshBody struct {
	RefreshToken string `json:"refresh_token"`
}

func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func CheckPassword(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}

// ValidatePassword rejects short or class-poor secrets.
func ValidatePassword(pw string) error {
	if len(pw) < 8 {
		return domain.WeakPassword("密码至少 8 位")
	}
	var upper, lower, digit bool
	for _, r := range pw {
		switch {
		case unicode.IsUpper(r):
			upper = true
		case unicode.IsLower(r):
			lower = true
		case unicode.IsDigit(r):
			digit = true
		}
	}
	if !upper || !lower || !digit {
		return domain.WeakPassword("密码须同时包含大小写字母与数字")
	}
	return nil
}

func (s *Service) Sign(user *domain.User, kind string, ttl time.Duration) (string, error) {
	now := platform.Now()
	claims := Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		Kind:     kind,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Subject:   user.Username,
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString(s.Secret)
}

func (s *Service) Parse(token string) (*Claims, error) {
	// Claims are parsed into a goroutine-local struct so concurrent Parse
	// calls can never overwrite each other's result. Returning a pointer
	// into shared state is what previously caused identity cross-over under
	// load (Alice's /auth/me returning Bob's id/username).
	claims := &Claims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, domain.Unauthorized("非法令牌算法")
		}
		return s.Secret, nil
	})
	if err != nil || !parsed.Valid {
		return nil, domain.Unauthorized("令牌无效或已过期")
	}
	return claims, nil
}

func (s *Service) pair(u *domain.User) (*TokenPair, error) {
	access, err := s.Sign(u, "access", AccessTTL)
	if err != nil {
		return nil, err
	}
	refresh, err := s.Sign(u, "refresh", RefreshTTL)
	if err != nil {
		return nil, err
	}
	return &TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(AccessTTL.Seconds()),
		User:         toPrincipal(u),
	}, nil
}

func toPrincipal(u *domain.User) *platform.UserPrincipal {
	return &platform.UserPrincipal{
		ID: u.ID, Username: u.Username, Email: u.Email,
		DisplayName: u.DisplayName, Role: u.Role,
	}
}

func (s *Service) Login(w http.ResponseWriter, r *http.Request) {
	var body loginBody
	if err := platform.DecodeJSON(r, &body); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	if body.Username == "" || body.Password == "" {
		platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "用户名与密码必填", nil))
		return
	}
	var u domain.User
	err := s.DB.Get(&u, `SELECT * FROM users WHERE username=$1 AND is_active=TRUE`, body.Username)
	if err != nil || CheckPassword(u.PasswordHash, body.Password) != nil {
		platform.WriteError(w, r, domain.Unauthorized("用户名或密码错误"))
		return
	}
	pair, err := s.pair(&u)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	platform.WriteData(w, http.StatusOK, pair)
}

func (s *Service) Register(w http.ResponseWriter, r *http.Request) {
	var body registerBody
	if err := platform.DecodeJSON(r, &body); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	body.Email = strings.ToLower(strings.TrimSpace(body.Email))
	body.DisplayName = strings.TrimSpace(body.DisplayName)
	if body.Username == "" || body.Email == "" {
		platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "用户名与邮箱必填", nil))
		return
	}
	if !strings.Contains(body.Email, "@") {
		platform.WriteError(w, r, domain.BadRequest(domain.CodeInvalidInput, "邮箱格式不正确", nil))
		return
	}
	if err := ValidatePassword(body.Password); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	if body.DisplayName == "" {
		body.DisplayName = body.Username
	}
	hash, err := HashPassword(body.Password)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	now := platform.Now()
	var id int64
	err = s.DB.QueryRow(`
		INSERT INTO users (username, email, password_hash, display_name, role, is_active, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,TRUE,$6,$6)
		RETURNING id`,
		body.Username, body.Email, hash, body.DisplayName, domain.RoleViewer, now,
	).Scan(&id)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			platform.WriteError(w, r, domain.Conflict(domain.CodeConflict, "用户名或邮箱已存在", nil))
			return
		}
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	u := domain.User{ID: id, Username: body.Username, Email: body.Email, DisplayName: body.DisplayName, Role: domain.RoleViewer}
	pair, err := s.pair(&u)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	platform.WriteData(w, http.StatusCreated, pair)
}

func (s *Service) Refresh(w http.ResponseWriter, r *http.Request) {
	var body refreshBody
	if err := platform.DecodeJSON(r, &body); err != nil {
		platform.WriteError(w, r, err)
		return
	}
	c, err := s.Parse(body.RefreshToken)
	if err != nil || c.Kind != "refresh" {
		platform.WriteError(w, r, domain.Unauthorized("刷新令牌无效"))
		return
	}
	var u domain.User
	if err := s.DB.Get(&u, `SELECT * FROM users WHERE id=$1 AND is_active=TRUE`, c.UserID); err != nil {
		platform.WriteError(w, r, domain.Unauthorized("用户不存在"))
		return
	}
	pair, err := s.pair(&u)
	if err != nil {
		platform.WriteError(w, r, domain.Internal(err))
		return
	}
	platform.WriteData(w, http.StatusOK, pair)
}

func (s *Service) Me(w http.ResponseWriter, r *http.Request) {
	p := platform.UserFrom(r)
	if p == nil {
		platform.WriteError(w, r, domain.Unauthorized("未登录"))
		return
	}
	var u domain.User
	if err := s.DB.Get(&u, `SELECT * FROM users WHERE id=$1`, p.ID); err != nil {
		platform.WriteError(w, r, domain.NotFound("用户不存在"))
		return
	}
	platform.WriteData(w, http.StatusOK, toPrincipal(&u))
}

func (s *Service) LoadUser(ctx context.Context, id int64) (*domain.User, error) {
	var u domain.User
	if err := s.DB.GetContext(ctx, &u, `SELECT * FROM users WHERE id=$1 AND is_active=TRUE`, id); err != nil {
		return nil, err
	}
	return &u, nil
}
