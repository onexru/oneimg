package controllers

import (
	"regexp"
	"time"
)

const (
	externalAuthFlowTTL      = 10 * time.Minute
	externalAuthCookiePrefix = "oneimg-auth-flow-"
	externalAuthMaxHTTPBody  = int64(2 << 20)
	casMaxResponseBody       = int64(1 << 20)
	casXMLNamespace          = "http://www.yale.edu/tp/cas"
)

var oidcClaimNameRegex = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.:-]{0,127}$`)

type externalIdentityProfile struct {
	Provider    string
	Issuer      string
	Subject     string
	Username    string
	Email       string
	DisplayName string
}
