package handlers

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/omnigate/services/auth/src/models"
	"github.com/omnigate/services/auth/src/repository"
)

// DigestRealm is the HTTP Digest Auth realm for ITSAPI camera authentication.
// This value is baked into each device's HA1 hash — changing it invalidates all digest credentials.
const DigestRealm = "omnigate"

type digestParams struct {
	Username  string
	Realm     string
	Nonce     string
	URI       string
	Response  string
	QOP       string
	NC        string
	CNonce    string
	Algorithm string
}

// Matches both quoted and unquoted values in Digest header fields.
var digestFieldRe = regexp.MustCompile(`(\w+)=(?:"([^"]*)"|([\w./+:@!#$%^&*()\-=]+))`)

func parseDigestHeader(header string) *digestParams {
	if !strings.HasPrefix(header, "Digest ") {
		return nil
	}
	p := &digestParams{}
	for _, m := range digestFieldRe.FindAllStringSubmatch(header[7:], -1) {
		val := m[2]
		if val == "" {
			val = m[3]
		}
		switch m[1] {
		case "username":
			p.Username = val
		case "realm":
			p.Realm = val
		case "nonce":
			p.Nonce = val
		case "uri":
			p.URI = val
		case "response":
			p.Response = val
		case "qop":
			p.QOP = val
		case "nc":
			p.NC = val
		case "cnonce":
			p.CNonce = val
		case "algorithm":
			p.Algorithm = val
		}
	}
	if p.Username == "" || p.Nonce == "" || p.Response == "" {
		return nil
	}
	return p
}

func md5hex(s string) string {
	h := md5.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

// ComputeHA1 computes the Digest HA1 = MD5(username:realm:password).
// Call this once when setting device credentials — store the result, not the password.
func ComputeHA1(username, password string) string {
	return md5hex(username + ":" + DigestRealm + ":" + password)
}

func computeDigestResponse(ha1, nonce, nc, cnonce, qop, method, uri string) string {
	ha2 := md5hex(method + ":" + uri)
	if qop == "auth" || qop == "auth-int" {
		return md5hex(ha1 + ":" + nonce + ":" + nc + ":" + cnonce + ":" + qop + ":" + ha2)
	}
	// RFC 2069 compatibility (no qop)
	return md5hex(ha1 + ":" + nonce + ":" + ha2)
}

func generateNonce() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// IssueDigestChallenge writes a 401 with a fresh WWW-Authenticate: Digest challenge.
// NGINX captures this header via auth_request_set and forwards it to the client.
func IssueDigestChallenge(c *gin.Context) {
	nonce := generateNonce()
	repository.StoreDigestNonce(nonce)
	c.Header(
		"WWW-Authenticate",
		fmt.Sprintf(`Digest realm="%s", qop="auth", nonce="%s", algorithm=MD5`, DigestRealm, nonce),
	)
	c.Status(401)
}

// ValidateDigestAuth parses and validates an Authorization: Digest header.
// Returns (sourceMetadata, ok). On failure the caller must issue a new challenge.
func ValidateDigestAuth(authHeader, method string) (models.SourceMetadata, bool) {
	p := parseDigestHeader(authHeader)
	if p == nil {
		return models.SourceMetadata{}, false
	}

	// Nonce must exist in Valkey (TTL-based, not consumed on use so cameras can
	// reuse within the TTL window without forcing a re-challenge on every request).
	if !repository.ValidateDigestNonce(p.Nonce) {
		return models.SourceMetadata{}, false
	}

	meta, ha1, ok := repository.FindDeviceByDigestUsername(p.Username)
	if !ok {
		return models.SourceMetadata{}, false
	}

	expected := computeDigestResponse(ha1, p.Nonce, p.NC, p.CNonce, p.QOP, method, p.URI)
	if !strings.EqualFold(expected, p.Response) {
		return models.SourceMetadata{}, false
	}

	return meta, true
}

// HandleSetDigestCredentials sets Digest Auth credentials for an existing API key.
// Accepts plaintext password, stores only HA1 = MD5(username:realm:password).
func HandleSetDigestCredentials(c *gin.Context) {
	id := c.Param("id")
	var b struct {
		DigestUsername string `json:"digest_username" binding:"required"`
		DigestPassword string `json:"digest_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&b); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	ha1 := ComputeHA1(b.DigestUsername, b.DigestPassword)
	username := b.DigestUsername

	result := repository.DB.WithContext(c.Request.Context()).
		Model(&models.APIKey{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"digest_username": &username,
			"digest_ha1":      &ha1,
		})

	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(404, gin.H{"error": "Key not found"})
		return
	}

	c.JSON(200, gin.H{"message": "Digest credentials set", "digest_username": b.DigestUsername})
}

// HandleClearDigestCredentials removes Digest Auth credentials from an API key.
func HandleClearDigestCredentials(c *gin.Context) {
	id := c.Param("id")
	result := repository.DB.WithContext(c.Request.Context()).
		Model(&models.APIKey{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"digest_username": nil,
			"digest_ha1":      nil,
		})
	if result.Error != nil {
		c.JSON(500, gin.H{"error": result.Error.Error()})
		return
	}
	if result.RowsAffected == 0 {
		c.JSON(404, gin.H{"error": "Key not found"})
		return
	}
	c.Status(204)
}
