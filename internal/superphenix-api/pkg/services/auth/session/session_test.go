package session

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/authentication/jwt"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/consts"
	"github.com/super-phenix/superphenix/internal/superphenix-api/internal/db"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/authentication"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/api/publicHttp/model"
	"github.com/super-phenix/superphenix/internal/superphenix-api/pkg/config"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	golangJwt "github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestRefreshAuth(t *testing.T) {
	// Setup sqlmock
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer sqlDB.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening gorm database", err)
	}

	// Override global db client
	oldClient := db.Client
	db.Client = gormDB
	defer func() { db.Client = oldClient }()

	userID := uuid.New()
	providerID := "kratos-id"
	cookieName := config.Global.Session.Cookies.Name

	// Helper to create valid JWT
	createToken := func(aud string) string {
		var tokenStr string
		if aud == jwt.AccessAudience {
			tokenStr, _ = jwt.CreateAccessToken(time.Hour, providerID)
		} else {
			token := jwt.NewRefreshToken(time.Hour, providerID)
			tokenStr, _ = token.SignedString(jwt.Secret)
		}
		return tokenStr
	}

	validAccessToken := createToken(jwt.AccessAudience)
	validRefreshToken := createToken(jwt.RefreshAudience)

	// Helper to create expired refresh token
	createExpiredToken := func() string {
		token := jwt.NewRefreshToken(-time.Hour, providerID)
		tokenStr, _ := token.SignedString(jwt.Secret)
		return tokenStr
	}
	expiredRefreshToken := createExpiredToken()

	tests := []struct {
		name           string
		setupRequest   func() *http.Request
		mockBehavior   func()
		expectedError  string
		checkContext   bool
		expectedResult bool // For Detection
	}{
		{
			name: "Detection - No cookie",
			setupRequest: func() *http.Request {
				return httptest.NewRequest("GET", "/", nil)
			},
			mockBehavior:   func() {},
			expectedResult: false,
		},
		{
			name: "Detection - Cookie exists",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.AddCookie(&http.Cookie{Name: cookieName, Value: "some-token"})
				return req
			},
			mockBehavior:   func() {},
			expectedResult: true,
		},
		{
			name: "Validation - Invalid token",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.AddCookie(&http.Cookie{Name: cookieName, Value: "invalid"})
				return req
			},
			mockBehavior:  func() {},
			expectedError: "token contains an invalid number of segments",
		},
		{
			name: "Validation - Expired token",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.AddCookie(&http.Cookie{Name: cookieName, Value: expiredRefreshToken})
				return req
			},
			mockBehavior:  func() {},
			expectedError: "Token is expired",
		},
		{
			name: "Validation - Wrong audience (Access instead of Refresh)",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.AddCookie(&http.Cookie{Name: cookieName, Value: validAccessToken})
				return req
			},
			mockBehavior:  func() {},
			expectedError: "the token is not a refresh token",
		},
		{
			name: "Validation - Success",
			setupRequest: func() *http.Request {
				req := httptest.NewRequest("GET", "/", nil)
				req.AddCookie(&http.Cookie{Name: cookieName, Value: validRefreshToken})
				return req
			},
			mockBehavior: func() {
				// RetrieveUserFromSession mocks
				mock.ExpectQuery(`SELECT \* FROM "users"`).
					WithArgs(db.KratosProvider, providerID, 1).
					WillReturnRows(sqlmock.NewRows([]string{"id", "provider_id", "provider", "is_active"}).
						AddRow(userID, providerID, db.KratosProvider, true))

				mock.ExpectQuery(`SELECT \* FROM "users"`).
					WithArgs(userID, userID, 1).
					WillReturnRows(sqlmock.NewRows([]string{"id", "is_active"}).
						AddRow(userID, true))
			},
			checkContext: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockBehavior()
			req := tt.setupRequest()
			rr := httptest.NewRecorder()

			if tt.expectedError == "" && tt.name[:9] == "Detection" {
				got := RefreshAuth.Detection(rr, req)
				assert.Equal(t, tt.expectedResult, got)
			} else {
				newReq, err := RefreshAuth.Validation(rr, req)
				if tt.expectedError != "" {
					assert.Error(t, err)
					assert.Contains(t, err.Error(), tt.expectedError)
				} else {
					assert.NoError(t, err)
					if tt.checkContext {
						assert.Equal(t, userID.String(), newReq.Context().Value(consts.ContextUserId))
					}
				}
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

const (
	sampleJWTID  = "5f3064d4-04a8-4090-92fa-87ae92aefed9"
	sampleUserId = "bc8dc5be-1e7b-4e07-b173-a43a222175b8"
)

func initMockDb(userActive bool) {
	mockDb, mock, _ := sqlmock.New()
	dialector := postgres.New(postgres.Config{
		Conn:       mockDb,
		DriverName: "postgres",
	})
	client, _ := gorm.Open(dialector, &gorm.Config{})

	db.Client = client

	rows := sqlmock.NewRows([]string{"Id", "IsActive"}).
		AddRow(sampleUserId, userActive)
	mock.ExpectQuery(`SELECT`).WillReturnRows(rows)
}

func TestGenerateTokens(t *testing.T) {

	type args struct {
		writer     *httptest.ResponseRecorder
		request    *http.Request
		subjectId  string
		userActive bool
	}

	type want struct {
		statusCode  int
		redirectUrl string
	}

	tests := []struct {
		name          string
		args          args
		want          want
		checkToken    bool
		checkInactive bool
	}{
		{
			name: "No Subject",
			args: args{
				writer:  httptest.NewRecorder(),
				request: httptest.NewRequest(http.MethodGet, "/session/token", nil),
			},
			want: want{
				statusCode: http.StatusInternalServerError,
			},
		},
		{
			name: "Valid Subject ID - User Inactive",
			args: args{
				writer:     httptest.NewRecorder(),
				request:    httptest.NewRequest(http.MethodGet, "/session/token", nil),
				subjectId:  sampleJWTID,
				userActive: false,
			},
			want: want{
				statusCode:  http.StatusFound,
				redirectUrl: fmt.Sprintf("%snot_active=%t", config.Global.Session.DefaultReturnUrl+"?", true),
			},
			checkToken: true,
		},
		{
			name: "Valid Subject ID - User Active",
			args: args{
				writer:     httptest.NewRecorder(),
				request:    httptest.NewRequest(http.MethodGet, "/session/token", nil),
				subjectId:  sampleJWTID,
				userActive: true,
			},
			want: want{
				statusCode: http.StatusFound,
			},
			checkToken: true,
		},
	}
	for _, tt := range tests {
		initMockDb(tt.args.userActive)
		t.Run(tt.name, func(t *testing.T) {
			// Add context if provided
			if tt.args.subjectId != "" {
				tt.args.request = tt.args.request.WithContext(context.WithValue(tt.args.request.Context(), consts.ContextSessionId, tt.args.subjectId))
				tt.args.request = tt.args.request.WithContext(context.WithValue(tt.args.request.Context(), consts.ContextUserId, sampleUserId))
			}

			New(&config.Global).GenerateTokens(tt.args.writer, tt.args.request)

			resp := tt.args.writer.Result()
			// Check response status code and text
			if resp.StatusCode != tt.want.statusCode {
				t.Errorf("GenerateTokens() = %v, want %v",
					resp.StatusCode,
					tt.want)
			}

			// If user is inactive, there is no token, instead check for correct redirect link
			if tt.checkToken && !tt.args.userActive {
				if resp.Header.Get("Location") != tt.want.redirectUrl {
					t.Errorf("Wrong redirection = %v, want %v", resp.Header.Get("Location"), tt.want.redirectUrl)
				}
				return
			}

			// If needed check token value
			if tt.checkToken {
				// Check refresh token
				cookies := resp.Cookies()
				if len(cookies) != 1 {
					t.Errorf("No session cookie present")
					return
				}
				refreshToken := cookies[0].Value
				refreshClaims, err := jwt.ParseTokenString(refreshToken)
				if err != nil {
					t.Errorf("Failed to verify generated Refresh JWT: %s", err.Error())
				}

				if refreshClaims["sub"] != sampleJWTID {
					t.Errorf("Failed to verify Refresh JWT sub ID = %v, want  %v", refreshClaims["sub"], sampleJWTID)
				}
				if refreshClaims.VerifyAudience(jwt.RefreshAudience, true) != true {
					t.Errorf("Failed to verify Refresh JWT audience = %v, want %v", refreshClaims["aud"], golangJwt.ClaimStrings{jwt.RefreshAudience})
				}

				// Check access token
				redirectLocation := resp.Header.Get("Location")
				redirectUrl, err := url.Parse(redirectLocation)
				if err != nil {
					t.Errorf("Failed to parse redirect URL: %s", err.Error())
				}
				accessToken := redirectUrl.Query().Get("session")
				accessClaims, err := jwt.ParseTokenString(accessToken)
				if err != nil {
					t.Errorf("Failed to verify generated Access JWT: %s", err.Error())
				}

				if accessClaims["sub"] != sampleJWTID {
					t.Errorf("Failed to verify Access JWT sub ID = %v, want  %v", accessClaims["sub"], sampleJWTID)
				}
				if accessClaims.VerifyAudience(jwt.AccessAudience, true) != true {
					t.Errorf("Failed to verify Access JWT audience = %v, want %v", accessClaims["aud"], golangJwt.ClaimStrings{jwt.AccessAudience})
				}
			}
		})
	}
}

func TestRefreshAuthMiddleware(t *testing.T) {
	const sampleJWTID = "id"
	const malformedJWT = "MALFORMEDTOKENMALFORMEDTOKEN"
	const sampleJWT = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJBR0FUIiwic3ViIjoiaWQiLCJleHAiOjE2Nzc4NTMyNTAsIm5iZi" +
		"I6MTY3Nzg1MzIzMSwiaWF0IjoxNjc3ODUzMjMxfQ.DBbaX8jekKGMmoGMtxcI4cy3k5ZHpgFnL7SiIYTn4-0"

	// Setup sqlmock
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer sqlDB.Close()

	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn: sqlDB,
	}), &gorm.Config{})
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening gorm database", err)
	}

	// Override global db client
	oldClient := db.Client
	db.Client = gormDB
	defer func() { db.Client = oldClient }()

	userID := uuid.New()

	type args struct {
		token func() string
	}

	type want struct {
		claimsPresent bool
		statusCode    int
	}

	tests := []struct {
		name         string
		args         args
		want         want
		mockBehavior func()
	}{
		{
			name:         "No Refresh Token",
			args:         args{token: func() string { return "" }},
			want:         want{claimsPresent: false, statusCode: http.StatusUnauthorized},
			mockBehavior: func() {},
		},
		{
			name: "Wrong Token Aud",
			args: args{
				token: func() string {
					token := jwt.NewJWTWithSubjectID(time.Second*10, sampleJWTID)
					jwtString, err := token.SignedString(jwt.Secret)
					if err != nil {
						t.Errorf("Failed to generate JWT: %s", err.Error())
					}
					return jwtString
				},
			},
			want:         want{claimsPresent: false, statusCode: http.StatusUnauthorized},
			mockBehavior: func() {},
		},
		{
			name:         "Malformed Token",
			args:         args{token: func() string { return malformedJWT }},
			want:         want{claimsPresent: false, statusCode: http.StatusUnauthorized},
			mockBehavior: func() {},
		},
		{
			name:         "Expired Token",
			args:         args{token: func() string { return sampleJWT }},
			want:         want{claimsPresent: false, statusCode: http.StatusUnauthorized},
			mockBehavior: func() {},
		},
		{
			name: "Valid Refresh Token",
			args: args{
				token: func() string {
					token := jwt.NewRefreshToken(time.Second*10, sampleJWTID)
					jwtString, err := token.SignedString(jwt.Secret)
					if err != nil {
						t.Errorf("Failed to generate JWT: %s", err.Error())
					}
					return jwtString
				},
			},
			want: want{claimsPresent: true, statusCode: http.StatusOK},
			mockBehavior: func() {
				mock.ExpectQuery(`SELECT \* FROM "users"`).
					WithArgs(db.KratosProvider, sampleJWTID, 1).
					WillReturnRows(sqlmock.NewRows([]string{"id", "provider_id", "provider", "is_active"}).
						AddRow(userID, sampleJWTID, db.KratosProvider, true))

				mock.ExpectQuery(`SELECT \* FROM "users"`).
					WithArgs(userID, userID, 1).
					WillReturnRows(sqlmock.NewRows([]string{"id", "is_active"}).
						AddRow(userID, true))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.mockBehavior()
			rr := httptest.NewRecorder()
			req, err := http.NewRequest("GET", "/session", nil)
			if err != nil {
				t.Fatal(err)
			}

			// Add cookie if subject id provided
			jwtString := tt.args.token()
			if jwtString != "" {
				req.AddCookie(&http.Cookie{
					Name:     config.Global.Session.Cookies.Name,
					Domain:   config.Global.Session.Cookies.Domain,
					SameSite: getSameSite(config.Global.Session.Cookies.SameSite),
					Path:     config.Global.Session.Cookies.Path,
					Expires:  time.Now().Add(config.Global.Session.RefreshValidity),
					MaxAge:   int(config.Global.Session.RefreshValidity.Seconds()),
					HttpOnly: true,
					Secure:   true,
					Value:    jwtString})
			}

			// Define test handler
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Claims presence check
				claims := r.Context().Value(jwt.ClaimsCtxKey)
				if (claims != nil) != tt.want.claimsPresent {
					t.Fatalf("claimsPresent = %v, want %v", claims != nil, tt.want.claimsPresent)
				}

				if claims != nil {
					jwtClaims := claims.(golangJwt.MapClaims)

					// Claims check
					if jwtClaims["iss"] != jwt.TokenIssuer {
						t.Errorf("Failed to verify JWT Token Issuer = %v, want %v", jwtClaims["iss"], jwt.TokenIssuer)
					}

					if jwtClaims["sub"] != sampleJWTID {
						t.Errorf("Failed to verify JWT Subject ID = %v, want %v", jwtClaims["sub"], sampleJWTID)
					}

					if jwtClaims.VerifyAudience(jwt.RefreshAudience, true) != true {
						t.Errorf("Failed to verify JWT audience = %v, want %v", jwtClaims["aud"], golangJwt.ClaimStrings{jwt.RefreshAudience})
					}
				}
				w.WriteHeader(http.StatusOK)
			})

			middleware := authentication.Authenticate(RefreshAuth)
			handler := middleware(nextHandler)
			handler.ServeHTTP(rr, req)

			resp := rr.Result()
			// Check response status code
			if resp.StatusCode != tt.want.statusCode {
				t.Errorf(
					"RefreshAuthMiddleware() = %v, want %v",
					resp.Status,
					tt.want.statusCode,
				)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRetrieveAccessToken(t *testing.T) {
	const sampleJWTID = "id"

	type args struct {
		writer     *httptest.ResponseRecorder
		request    *http.Request
		subjectId  string
		userActive bool
	}

	type want struct {
		statusCode int
		status     string
	}

	tests := []struct {
		name       string
		args       args
		want       want
		checkToken bool
	}{
		{
			name: "No Subject",
			args: args{
				writer:  httptest.NewRecorder(),
				request: httptest.NewRequest(http.MethodGet, "/session", nil),
			},
			want: want{
				statusCode: http.StatusInternalServerError,
				status:     http.StatusText(http.StatusInternalServerError),
			},
		},
		{
			name: "Valid Subject ID - User Inactive",
			args: args{
				writer:     httptest.NewRecorder(),
				request:    httptest.NewRequest(http.MethodGet, "/session", nil),
				subjectId:  sampleJWTID,
				userActive: false,
			},
			want: want{
				statusCode: http.StatusFound,
				status:     http.StatusText(http.StatusFound),
			},
			checkToken: false,
		},
		{
			name: "Valid Subject ID - User Active",
			args: args{
				writer:     httptest.NewRecorder(),
				request:    httptest.NewRequest(http.MethodGet, "/session", nil),
				subjectId:  sampleJWTID,
				userActive: true,
			},
			want: want{
				statusCode: http.StatusOK,
				status:     http.StatusText(http.StatusOK),
			},
			checkToken: true,
		},
	}
	for _, tt := range tests {
		initMockDb(tt.args.userActive)
		t.Run(tt.name, func(t *testing.T) {
			// Add context if provided
			if tt.args.subjectId != "" {
				tt.args.request = tt.args.request.WithContext(context.WithValue(tt.args.request.Context(), consts.ContextSessionId, tt.args.subjectId))
				tt.args.request = tt.args.request.WithContext(context.WithValue(tt.args.request.Context(), consts.ContextUserId, sampleUserId))
			}

			New(&config.Global).RetrieveAccessToken(tt.args.writer, tt.args.request)

			resp := tt.args.writer.Result()
			// Check response status code and text
			if resp.StatusCode != tt.want.statusCode && resp.Status != tt.want.status {
				t.Errorf("GenerateTokens() = %v, want %v", want{
					resp.StatusCode,
					resp.Status,
				}, tt.want)
			}

			// If needed check token value
			if tt.checkToken {
				// Check access token
				var response struct {
					Session string        `json:"session"`
					User    model.APIUser `json:"user"`
				}
				err := json.NewDecoder(tt.args.writer.Body).Decode(&response)

				accessClaims, err := jwt.ParseTokenString(response.Session)
				if err != nil {
					t.Errorf("Failed to verify generated Access JWT: %s", err.Error())
				}

				if accessClaims["sub"] != sampleJWTID {
					t.Errorf("Failed to verify Access JWT sub ID = %v, want  %v", accessClaims["sub"], sampleJWTID)
				}
				if accessClaims.VerifyAudience(jwt.AccessAudience, true) != true {
					t.Errorf("Failed to verify Access JWT audience = %v, want %v", accessClaims["aud"], golangJwt.ClaimStrings{jwt.AccessAudience})
				}
			}
		})
	}
}

func Test_getSameSite(t *testing.T) {
	type args struct {
		sameSite string
	}
	tests := []struct {
		name string
		args args
		want http.SameSite
	}{
		{name: "Strict", args: args{sameSite: "Strict"}, want: http.SameSiteStrictMode},
		{name: "Lax", args: args{sameSite: "Lax"}, want: http.SameSiteLaxMode},
		{name: "None", args: args{sameSite: "None"}, want: http.SameSiteNoneMode},
		{name: "Empty input", args: args{sameSite: ""}, want: http.SameSiteDefaultMode},
		{name: "Unknown value", args: args{sameSite: "Unknown"}, want: http.SameSiteDefaultMode},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getSameSite(tt.args.sameSite); got != tt.want {
				t.Errorf("getSameSite() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseReturnUrl(t *testing.T) {
	// Backup and restore configuration
	oldAllowedOrigins := config.Global.Session.Cors.AllowedOrigins
	oldAllowUnsafeWildcard := config.Global.Session.Cors.AllowUnsafeWildcard
	defer func() {
		config.Global.Session.Cors.AllowedOrigins = oldAllowedOrigins
		config.Global.Session.Cors.AllowUnsafeWildcard = oldAllowUnsafeWildcard
	}()

	type args struct {
		returnTo            string
		allowedOrigins      []string
		allowUnsafeWildcard bool
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "Empty input",
			args: args{returnTo: "", allowedOrigins: []string{"http://localhost:4200"}},
			want: "http://localhost:4200/callback?",
		},
		{
			name: `Wildcard "*" is ignored when allowUnsafeWildcard is false`,
			args: args{returnTo: "http://attacker.example/steal", allowedOrigins: []string{"*"}},
			want: "http://localhost:4200/callback?",
		},
		{
			name: `Wildcard "*" is honoured when allowUnsafeWildcard is true`,
			args: args{returnTo: "http://localhost:4201/private/home", allowedOrigins: []string{"*"}, allowUnsafeWildcard: true},
			want: "http://localhost:4201/callback?return_to=/private/home&",
		},
		{
			name: "Valid input - allowed with exact match",
			args: args{returnTo: "http://localhost:4201/private/home", allowedOrigins: []string{"http://localhost:4201"}},
			want: "http://localhost:4201/callback?return_to=/private/home&",
		},
		{
			name: "Valid input - allowed with case-insensitive host match",
			args: args{returnTo: "http://LOCALHOST:4201/private/home", allowedOrigins: []string{"http://localhost:4201"}},
			want: "http://LOCALHOST:4201/callback?return_to=/private/home&",
		},
		{
			name: "Valid input - NOT allowed",
			args: args{returnTo: "http://attacker.com/steal", allowedOrigins: []string{"http://localhost:4201"}},
			want: "http://localhost:4200/callback?",
		},
		{
			name: "Malformed url",
			args: args{returnTo: "localhost:4201/private/home", allowedOrigins: []string{"http://localhost:4201"}},
			want: "http://localhost:4200/callback?",
		},
		{
			name: "Valid input - no path (root)",
			args: args{returnTo: "http://localhost:4200", allowedOrigins: []string{"http://localhost:4200"}},
			want: "http://localhost:4200/callback?return_to=/&",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Global.Session.Cors.AllowedOrigins = tt.args.allowedOrigins
			config.Global.Session.Cors.AllowUnsafeWildcard = tt.args.allowUnsafeWildcard
			if got := parseReturnUrl(context.Background(), tt.args.returnTo); got != tt.want {
				t.Errorf("parseReturnUrl() = %v, want %v", got, tt.want)
			}
		})
	}
}
