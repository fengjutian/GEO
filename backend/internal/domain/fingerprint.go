package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Fingerprint identifies an issue across scans using the site, normalised page
// URL and rule code.
func Fingerprint(siteID uint, pageURL, ruleCode string) string {
	normalized := strings.ToLower(strings.TrimSpace(pageURL))
	normalized = strings.TrimSuffix(normalized, "/")
	sum := sha256.Sum256([]byte(fmt.Sprintf("%d|%s|%s", siteID, normalized, ruleCode)))
	return hex.EncodeToString(sum[:])
}
