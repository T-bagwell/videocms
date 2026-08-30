package api

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"html"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"github.com/crewjam/saml/samlidp"

	"videocms/backend/internal/config"
)

func TestSAMLLogin(t *testing.T) {
	// Stand up a real SAML IdP (crewjam's test IdP) with its own key pair.
	idpKey, idpCert := newTestKeyPair(t, "VideoCMS Test IdP")
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	idpURL := "http://" + listener.Addr().String()
	parsed, _ := url.Parse(idpURL)
	idp, err := samlidp.New(samlidp.Options{
		URL:         *parsed,
		Key:         idpKey,
		Signer:      idpKey,
		Certificate: idpCert,
		Store:       &samlidp.MemoryStore{},
	})
	if err != nil {
		t.Fatal(err)
	}
	idpServer := &http.Server{Handler: idp}
	go func() { _ = idpServer.Serve(listener) }()
	defer func() { _ = idpServer.Close() }()

	// SP key pair on disk for the backend.
	spKey, spCert := newTestKeyPair(t, "VideoCMS SAML SP")
	spDir := t.TempDir()
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: spCert.Raw})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(spKey)})
	certPath := filepath.Join(spDir, "sp.crt")
	keyPath := filepath.Join(spDir, "sp.key")
	if err := os.WriteFile(certPath, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		t.Fatal(err)
	}

	env := newIntegrationEnv(t, func(c *config.Config) {
		c.SAMLIDPMetadataURL = idpURL + "/metadata"
		c.SAMLSPCert = certPath
		c.SAMLSPKey = keyPath
	})
	noRedirect := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	entityID := env.server.URL + "/api/auth/saml/acs"
	env.app.cfg.SAMLSPEntityID = entityID
	env.app.cfg.SAMLACSURL = entityID

	// Register the SP metadata with the IdP.
	metaRes, err := http.Get(env.server.URL + "/api/auth/saml/metadata")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = metaRes.Body.Close() }()
	metaXML, _ := io.ReadAll(metaRes.Body)
	if metaRes.StatusCode != http.StatusOK {
		t.Fatalf("SP metadata status = %d: %s", metaRes.StatusCode, metaXML)
	}
	req, _ := http.NewRequest("PUT", idpURL+"/services/"+url.PathEscape(entityID), strings.NewReader(string(metaXML)))
	req.Header.Set("Content-Type", "text/xml")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("register SP status = %d", res.StatusCode)
	}

	// Register an IdP user and a shortcut that authenticates to our SP.
	userBody, _ := json.Marshal(map[string]string{
		"email": "bob@example.com", "name": "Bob", "password": "secret",
	})
	req, _ = http.NewRequest("PUT", idpURL+"/users/bob", strings.NewReader(string(userBody)))
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("register user status = %d", res.StatusCode)
	}
	shortcutBody, _ := json.Marshal(map[string]string{"service_provider": entityID})
	req, _ = http.NewRequest("PUT", idpURL+"/shortcuts/videocms", strings.NewReader(string(shortcutBody)))
	req.Header.Set("Content-Type", "application/json")
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("register shortcut status = %d", res.StatusCode)
	}

	// Start: /api/auth/saml/login must redirect to the IdP SSO endpoint.
	loginRes, err := noRedirect.Get(env.server.URL + "/api/auth/saml/login")
	if err != nil {
		t.Fatal(err)
	}
	_ = loginRes.Body.Close()
	if loginRes.StatusCode != http.StatusFound {
		t.Fatalf("saml login status = %d", loginRes.StatusCode)
	}
	loc := loginRes.Header.Get("Location")
	if !strings.Contains(loc, "/sso") || !strings.Contains(loc, "SAMLRequest=") {
		t.Fatalf("unexpected saml login redirect: %s", loc)
	}

	// Log in at the IdP with credentials (cookie jar keeps the session),
	// then trigger the IdP-initiated flow for our SP.
	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	res, err = client.PostForm(idpURL+"/login", url.Values{"user": {"bob"}, "password": {"secret"}})
	if err != nil {
		t.Fatal(err)
	}
	form, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("idp post-login status = %d: %s", res.StatusCode, form)
	}
	res, err = client.Get(idpURL + "/login/videocms")
	if err != nil {
		t.Fatal(err)
	}
	form, _ = io.ReadAll(res.Body)
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("idp initiated flow status = %d: %s", res.StatusCode, form)
	}
	action, samlResponse := parseSAMLPOSTForm(string(form))
	if action == "" || samlResponse == "" {
		t.Fatalf("idp did not emit SAMLResponse form: %s", form)
	}
	// A browser submits the DOM attribute value, so HTML entities in the
	// template output must be decoded before posting.
	samlResponse = html.UnescapeString(samlResponse)

	// Deliver the response to our ACS and expect a frontend redirect with token.
	acsRes, err := client.PostForm(action, url.Values{"SAMLResponse": {samlResponse}, "RelayState": {""}})
	if err != nil {
		t.Fatal(err)
	}
	acsBody, _ := io.ReadAll(acsRes.Body)
	_ = acsRes.Body.Close()
	if acsRes.StatusCode != http.StatusFound {
		t.Fatalf("acs status = %d: %s", acsRes.StatusCode, acsBody)
	}
	acsLoc := acsRes.Header.Get("Location")
	if !strings.Contains(acsLoc, "/login?sso_token=") {
		t.Fatalf("acs redirect missing token: %s", acsLoc)
	}
	token := strings.TrimPrefix(acsLoc, "/login?sso_token=")
	token, err = url.QueryUnescape(token)
	if err != nil {
		t.Fatal(err)
	}

	// The session token works and resolves to the SAML user.
	status, d := doJSON(t, "GET", env.server.URL+"/api/auth/me", token, nil)
	if status != http.StatusOK {
		t.Fatalf("me status = %d", status)
	}
	if username, _ := d["username"].(string); username != "bob@example.com" {
		t.Fatalf("username = %q, want bob@example.com", username)
	}
	var sub string
	if err := env.pool.QueryRow(context.Background(),
		`SELECT oauth_sub FROM users WHERE username='bob@example.com'`).Scan(&sub); err != nil {
		t.Fatalf("load oauth_sub: %v", err)
	}
	if sub != "saml:bob@example.com" {
		t.Fatalf("oauth_sub = %q, want saml:bob@example.com", sub)
	}
}

func TestSAMLAttributeHelpers(t *testing.T) {
	if !samlHasAdminRole([]string{"user", "Admin"}) {
		t.Fatal("expected Admin role to be recognized")
	}
	if samlHasAdminRole([]string{"user"}) {
		t.Fatal("expected plain user role to be ignored")
	}
	email, display, roles := samlAttributes(&saml.Assertion{})
	if email != "" || display != "" || roles != nil {
		t.Fatalf("empty assertion yielded %q %q %v", email, display, roles)
	}
}

// newTestKeyPair returns an RSA key and a self-signed certificate.
func newTestKeyPair(t *testing.T, cn string) (*rsa.PrivateKey, *x509.Certificate) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: cn},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return key, cert
}

var samlFormRe = regexp.MustCompile(`<form[^>]*action="([^"]+)"[^>]*>.*?<input type="hidden" name="SAMLResponse" value="([^"]*)"`)

func parseSAMLPOSTForm(html string) (action, samlResponse string) {
	m := samlFormRe.FindStringSubmatch(html)
	if len(m) != 3 {
		return "", ""
	}
	action = strings.ReplaceAll(m[1], "&amp;", "&")
	return action, m[2]
}
