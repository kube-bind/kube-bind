package cookie

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"time"
)

func MakeCookie(req *http.Request, name string, value []byte, expiration time.Duration) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    base64.RawURLEncoding.EncodeToString(value),
		Path:     "/", // TODO: make configurable
		Domain:   "",  // TODO: add domain support
		Expires:  time.Now().Add(expiration),
		HttpOnly: true, // TODO: make configurable
		// setting to false so it works over http://localhost
		Secure:   false,             // TODO: make configurable
		SameSite: ParseSameSite(""), // TODO: make configurable
	}
}

func ParseSameSite(v string) http.SameSite {
	switch v {
	case "lax":
		return http.SameSiteLaxMode
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	case "":
		return 0
	default:
		panic(fmt.Sprintf("Invalid value for SameSite: %s", v))
	}
}
