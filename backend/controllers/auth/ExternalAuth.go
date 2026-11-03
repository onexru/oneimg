package auth

import (
	"time"
)

const (
	externalAuthFlowTTL      = 10 * time.Minute
	externalAuthCookiePrefix = "oneimg-auth-flow-"
	externalAuthMaxHTTPBody  = int64(2 << 20)
	casMaxResponseBody       = int64(1 << 20)
	casXMLNamespace          = "http://www.yale.edu/tp/cas"
)

type externalIdentityProfile struct {
	Provider    string
	Issuer      string
	Subject     string
	Username    string
	Email       string
	DisplayName string
}
