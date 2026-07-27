package jwt

import (
	"errors"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

const (
	sampleJWT = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJBR0FUIiwic3ViIjoiaWQiLCJleHAiOjE2Nzc4NTMyNTAsIm5iZi" +
		"I6MTY3Nzg1MzIzMSwiaWF0IjoxNjc3ODUzMjMxfQ.DBbaX8jekKGMmoGMtxcI4cy3k5ZHpgFnL7SiIYTn4-0"
	malformedJWT = "MALFORMEDTOKENMALFORMEDTOKEN"
	sampleJWTID  = "id"
)

func TestParseTokenString(t *testing.T) {
	token := NewJWTWithSubjectID(time.Second*100, sampleJWTID)
	jwtstr, err := token.SignedString(Secret)
	if err != nil {
		t.Errorf("Failed to convert JWT to STR: %s", err.Error())
	}

	claims, err := ParseTokenString(jwtstr)
	if err != nil {
		t.Errorf("Failed to parse token string: %s", err)
	}

	if claims["iss"] != TokenIssuer {
		t.Errorf("Wrong token issuer, expected %s and got %s", TokenIssuer, claims["iss"])
	}

	if claims["sub"] != sampleJWTID {
		t.Errorf("Wrong subject ID, expected %s and got %s", sampleJWTID, claims["sub"])
	}
}

func TestParseExpiredToken(t *testing.T) {
	_, err := ParseTokenString(sampleJWT)
	if !errors.Is(err, jwt.ErrTokenExpired) && err != nil {
		t.Errorf("Expected expired token, got %s", err.Error())
	}

	if err == nil {
		t.Errorf("Expected error and got nothing, token should be expired")
	}
}

func TestParseTokenMalformed(t *testing.T) {
	_, err := ParseTokenString(malformedJWT)
	if err == nil {
		t.Errorf("Expected malformed token not to be parsed")
	}
}

func TestIsAudience(t *testing.T) {
	type args struct {
		claims   jwt.MapClaims
		audience string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "Correct audience",
			args: args{
				claims: jwt.MapClaims{
					"aud": RefreshAudience,
				},
				audience: RefreshAudience,
			},
			want: true,
		},
		{
			name: "Wrong audience",
			args: args{
				claims: jwt.MapClaims{
					"aud": RefreshAudience,
				},
				audience: AccessAudience,
			},
			want: false,
		},
		{
			name: "Invalid audience",
			args: args{
				claims: jwt.MapClaims{
					"aud": RefreshAudience,
				},
				audience: "test",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAudience(tt.args.claims, tt.args.audience); got != tt.want {
				t.Errorf("IsAudience() = %v, want %v", got, tt.want)
			}
		})
	}
}
