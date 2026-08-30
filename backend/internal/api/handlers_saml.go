package api

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/crewjam/saml"
	dsig "github.com/russellhaering/goxmldsig"

	"videocms/backend/internal/auth"
)

func (a *App) samlConfigured() bool {
	return a.cfg.SAMLIDPMetadataURL != "" && a.cfg.SAMLSPCert != "" && a.cfg.SAMLSPKey != ""
}

// loadSAMLSP builds (and caches) the SAML ServiceProvider from the config:
// SP key pair + IdP metadata, with the ACS URL as the default entity ID.
func (a *App) loadSAMLSP() (*saml.ServiceProvider, error) {
	a.samlMu.Lock()
	defer a.samlMu.Unlock()
	if a.samlSP != nil {
		return a.samlSP, nil
	}
	keyPair, err := tls.LoadX509KeyPair(a.cfg.SAMLSPCert, a.cfg.SAMLSPKey)
	if err != nil {
		return nil, fmt.Errorf("load SAML SP key pair: %w", err)
	}
	cert, err := x509.ParseCertificate(keyPair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("parse SAML SP certificate: %w", err)
	}
	var signer crypto.Signer
	switch k := keyPair.PrivateKey.(type) {
	case *rsa.PrivateKey:
		signer = k
	case *ecdsa.PrivateKey:
		signer = k
	default:
		return nil, fmt.Errorf("unsupported SAML SP private key type %T", keyPair.PrivateKey)
	}

	metaReq, err := http.NewRequestWithContext(context.Background(), http.MethodGet, a.cfg.SAMLIDPMetadataURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build IdP metadata request: %w", err)
	}
	metaRes, err := http.DefaultClient.Do(metaReq)
	if err != nil {
		return nil, fmt.Errorf("fetch IdP metadata: %w", err)
	}
	defer func() { _ = metaRes.Body.Close() }()
	if metaRes.StatusCode < 200 || metaRes.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch IdP metadata: HTTP %d", metaRes.StatusCode)
	}
	rawMeta, err := io.ReadAll(io.LimitReader(metaRes.Body, 5<<20))
	if err != nil {
		return nil, fmt.Errorf("read IdP metadata: %w", err)
	}
	var idpMeta saml.EntityDescriptor
	if err := xml.Unmarshal(rawMeta, &idpMeta); err != nil {
		return nil, fmt.Errorf("parse IdP metadata: %w", err)
	}

	acsURL := a.cfg.SAMLACSURL
	if acsURL == "" {
		acsURL = a.cfg.SAMLSPEntityID
	}
	if acsURL == "" {
		return nil, fmt.Errorf("set SAML_ACS_URL or SAML_SP_ENTITY_ID")
	}
	acs, err := url.Parse(acsURL)
	if err != nil {
		return nil, fmt.Errorf("parse ACS URL: %w", err)
	}
	entityID := a.cfg.SAMLSPEntityID
	if entityID == "" {
		entityID = acsURL
	}
	metaURL, _ := url.Parse(entityID)

	sp := &saml.ServiceProvider{
		EntityID:          entityID,
		Key:               signer,
		Certificate:       cert,
		MetadataURL:       *metaURL,
		AcsURL:            *acs,
		IDPMetadata:       &idpMeta,
		SignatureMethod:   dsig.RSASHA256SignatureMethod,
		AllowIDPInitiated: true,
		AuthnNameIDFormat: saml.PersistentNameIDFormat,
	}
	a.samlSP = sp
	return sp, nil
}

// GET /api/auth/saml/login — redirect to the IdP with an AuthnRequest.
func (a *App) samlLogin(w http.ResponseWriter, r *http.Request) {
	if !a.samlConfigured() {
		writeErr(w, http.StatusBadRequest, "SAML is not configured (set SAML_IDP_METADATA_URL, SAML_SP_CERT, SAML_SP_KEY)")
		return
	}
	sp, err := a.loadSAMLSP()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "SAML setup failed: "+err.Error())
		return
	}
	binding := saml.HTTPRedirectBinding
	location := sp.GetSSOBindingLocation(binding)
	if location == "" {
		binding = saml.HTTPPostBinding
		location = sp.GetSSOBindingLocation(binding)
	}
	if location == "" {
		writeErr(w, http.StatusBadGateway, "IdP metadata has no usable SingleSignOnService")
		return
	}
	authReq, err := sp.MakeAuthenticationRequest(location, binding, saml.HTTPPostBinding)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "build AuthnRequest failed: "+err.Error())
		return
	}
	if binding == saml.HTTPRedirectBinding {
		redirectURL, err := authReq.Redirect("", sp)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "build redirect failed: "+err.Error())
			return
		}
		http.Redirect(w, r, redirectURL.String(), http.StatusFound)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html><html><body>%s</body></html>`, authReq.Post(""))
}

// POST /api/auth/saml/acs — consume the SAMLResponse, resolve the user and
// redirect to the frontend with a session token.
func (a *App) samlACS(w http.ResponseWriter, r *http.Request) {
	if !a.samlConfigured() {
		writeErr(w, http.StatusBadRequest, "SAML is not configured")
		return
	}
	if err := r.ParseForm(); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid SAML response form")
		return
	}
	sp, err := a.loadSAMLSP()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "SAML setup failed: "+err.Error())
		return
	}
	assertion, err := sp.ParseResponse(r, []string{""})
	if err != nil {
		var invalid *saml.InvalidResponseError
		if errors.As(err, &invalid) && invalid.PrivateErr != nil {
			log.Printf("saml acs rejected: %v", invalid.PrivateErr)
		}
		writeErr(w, http.StatusUnauthorized, "SAML assertion rejected: "+err.Error())
		return
	}
	nameID := ""
	if assertion.Subject != nil && assertion.Subject.NameID != nil {
		nameID = assertion.Subject.NameID.Value
	}
	if nameID == "" {
		writeErr(w, http.StatusUnauthorized, "SAML assertion has no NameID")
		return
	}
	email, display, roles := samlAttributes(assertion)
	if email == "" && strings.Contains(nameID, "@") {
		email = nameID
	}
	sub := "saml:" + nameID
	user, err := a.upsertOIDCUser(r.Context(), sub, email, display, email)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "resolve SAML user failed")
		return
	}
	if samlHasAdminRole(roles) && user.Role != "admin" {
		if _, err := a.pool.Exec(r.Context(),
			`UPDATE users SET role='admin' WHERE id=$1`, user.ID); err == nil {
			user.Role = "admin"
		}
	}
	jwt, err := auth.Sign(a.cfg.JWTSecret, user)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "sign session failed")
		return
	}
	http.Redirect(w, r, "/login?sso_token="+url.QueryEscape(jwt), http.StatusFound)
}

// GET /api/auth/saml/metadata — SP metadata for configuring the IdP.
func (a *App) samlMetadata(w http.ResponseWriter, r *http.Request) {
	if !a.samlConfigured() {
		writeErr(w, http.StatusBadRequest, "SAML is not configured")
		return
	}
	sp, err := a.loadSAMLSP()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "SAML setup failed: "+err.Error())
		return
	}
	buf, err := xml.MarshalIndent(sp.Metadata(), "", "  ")
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "build SP metadata failed")
		return
	}
	w.Header().Set("Content-Type", "application/samlmetadata+xml")
	_, _ = w.Write(buf)
}

// samlAttributes extracts email/display-name/roles from an assertion.
func samlAttributes(a *saml.Assertion) (email, display string, roles []string) {
	for _, stmt := range a.AttributeStatements {
		for _, attr := range stmt.Attributes {
			name := strings.ToLower(attr.Name)
			for _, v := range attr.Values {
				value := v.Value
				switch name {
				case "email", "mail", "emailaddress":
					if email == "" {
						email = value
					}
				case "displayname", "name", "cn":
					if display == "" {
						display = value
					}
				case "role", "roles", "group", "groups":
					roles = append(roles, value)
				}
			}
		}
	}
	return email, display, roles
}

func samlHasAdminRole(roles []string) bool {
	for _, r := range roles {
		if strings.EqualFold(strings.TrimSpace(r), "admin") ||
			strings.Contains(strings.ToLower(r), "admin") {
			return true
		}
	}
	return false
}
