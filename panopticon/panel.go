package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/skip2/go-qrcode"
	"golang.org/x/crypto/argon2"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	panelAddressEnvironment      = "PANOPTICON_PANEL_ADDR"
	panelPasswordHashEnvironment = "PANOPTICON_PANEL_PASSWORD_HASH"
	panelSessionKeyEnvironment   = "PANOPTICON_PANEL_SESSION_KEY"

	panelBasePath          = "/panel/"
	panelLoginPath         = "/panel/login"
	panelCookieName        = "__Host-panopticon_session"
	panelSessionLifetime   = 8 * time.Hour
	panelSessionIdleLimit  = 30 * time.Minute
	panelLoginFailureLimit = 5
	panelLoginFailureAge   = 15 * time.Minute
	panelMaximumBodyBytes  = 64 << 10

	panelMaximumSessions       = 4096
	panelMaximumLoginAddresses = 1024
	panelDockerHealthVision    = "docker.health"
	panelDockerHealthForm      = 1
	oraclePairingQRSchema      = "panoptes.oracle.pair"
	oraclePairingQRVersion     = 1
)

var panelSigilPattern = regexp.MustCompile(
	`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`,
)

var errPanelEyeNotFound = errors.New("Eye was not found")

// panelHTMX is htmx 2.0.10, vendored so the panel can use a strict
// self-hosted Content Security Policy.
//
//go:embed panel_htmx.min.js
var panelHTMX []byte

//go:embed panel.css
var panelCSS []byte

//go:embed panel.js
var panelJavaScript []byte

//go:embed panel_login_decoration.png
var panelLoginDecoration []byte

type argon2idPasswordHash struct {
	memory      uint32
	iterations  uint32
	parallelism uint8
	salt        []byte
	key         []byte
}

type panelConfig struct {
	address            string
	passwordHash       argon2idPasswordHash
	sessionKey         []byte
	sessionLifetime    time.Duration
	sessionIdleTimeout time.Duration
}

type panelSession struct {
	csrfToken string
	createdAt time.Time
	lastSeen  time.Time
}

type panelLoginFailures struct {
	count        int
	windowOpened time.Time
	blockedUntil time.Time
	lastSeen     time.Time
}

type panelAuthenticator struct {
	passwordHash argon2idPasswordHash
	sessionKey   []byte

	sessionLifetime    time.Duration
	sessionIdleTimeout time.Duration
	now                func() time.Time

	mu       sync.Mutex
	sessions map[string]panelSession
	failures map[string]panelLoginFailures
	verify   chan struct{}
}

type panelSessionContextKey struct{}

type controlPanel struct {
	panoptes  *PanoptesServer
	address   string
	auth      *panelAuthenticator
	templates *template.Template
	http      *http.Server
}

type panelEye struct {
	ID        string
	FirstSeen time.Time
	LastSeen  time.Time
	Online    bool
	Sigils    []string
}

type panelGaze struct {
	Sigil                    string
	Turn                     uint64
	Awake                    bool
	Vision                   string
	Form                     uint32
	Target                   string
	ReconcileIntervalSeconds string
	StartingGraceSeconds     string
}

type panelNavigationData struct {
	CSRF       string
	CurrentTab string
}

type panelOmen struct {
	EyeID      string
	GazeSigil  string
	GazeTurn   uint64
	BefallenAt time.Time
	ReceivedAt time.Time
}

type panelOracle struct {
	ID        string
	PairedAt  time.Time
	RevokedAt *time.Time
}

type panelPagination struct {
	Page         int
	TotalPages   int
	Total        int
	HasPrevious  bool
	HasNext      bool
	PreviousPage int
	NextPage     int
}

type panelEyesData struct {
	panelNavigationData
	Eyes       []panelEye
	Pagination panelPagination
	Query      string
	Summary    panelEyeSummary
}

type panelEyeSummary struct {
	All        int
	Open       int
	Closed     int
	SigilCount int
}

type panelEyeData struct {
	panelNavigationData
	Eye                      panelEye
	Visions                  []VisionRecord
	ConfigurableDockerHealth bool
	Gazes                    []panelGaze
	Message                  string
	Error                    string
}

type panelLoginData struct {
	Error string
	Next  string
}

type panelOraclesData struct {
	panelNavigationData
	Oracles []panelOracle
}

type panelOmensData struct {
	panelNavigationData
	Omens []panelOmen
}

type panelSealOutcome struct {
	Seal      string
	ExpiresAt time.Time
	Error     string
}

type panelOracleSealOutcome struct {
	Endpoint      string
	Seal          string
	ExpiresAt     time.Time
	QRCodeDataURL template.URL
	Error         string
}

type panelSealHistoryItem struct {
	Kind         string
	ForgedAt     time.Time
	ExpiresAt    time.Time
	Availability string
	Consumed     bool
	ConsumedAt   time.Time
}

type panelSealsData struct {
	panelNavigationData
	Eye         panelSealOutcome
	Oracle      panelOracleSealOutcome
	SealHistory []panelSealHistoryItem
}

type oraclePairingQRPayload struct {
	Schema        string `json:"schema"`
	Version       int    `json:"version"`
	Endpoint      string `json:"endpoint"`
	OracleSeal    string `json:"oracle_seal"`
	ExpiresAtUnix int64  `json:"expires_at_unix"`
}

func loadControlPanelConfig() (*panelConfig, error) {
	address := strings.TrimSpace(os.Getenv(panelAddressEnvironment))
	if address == "" {
		return nil, nil
	}

	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf(
			"%s must be a host:port address: %w",
			panelAddressEnvironment,
			err,
		)
	}
	if port == "" {
		return nil, fmt.Errorf(
			"%s must be a host:port address",
			panelAddressEnvironment,
		)
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return nil, fmt.Errorf(
			"%s must use a port between 1 and 65535",
			panelAddressEnvironment,
		)
	}

	passwordHash, err := parseArgon2idPasswordHash(
		strings.TrimSpace(os.Getenv(panelPasswordHashEnvironment)),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"load %s: %w",
			panelPasswordHashEnvironment,
			err,
		)
	}

	sessionKey, err := decodePanelSessionKey(
		strings.TrimSpace(os.Getenv(panelSessionKeyEnvironment)),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"load %s: %w",
			panelSessionKeyEnvironment,
			err,
		)
	}

	return &panelConfig{
		address:            address,
		passwordHash:       passwordHash,
		sessionKey:         sessionKey,
		sessionLifetime:    panelSessionLifetime,
		sessionIdleTimeout: panelSessionIdleLimit,
	}, nil
}

func parseArgon2idPasswordHash(encoded string) (argon2idPasswordHash, error) {
	if encoded == "" {
		return argon2idPasswordHash{}, errors.New("value is required")
	}

	parts := strings.Split(encoded, "$")
	if len(parts) != 6 ||
		parts[0] != "" ||
		parts[1] != "argon2id" ||
		parts[2] != "v=19" {
		return argon2idPasswordHash{}, errors.New(
			"value must be an Argon2id PHC string",
		)
	}

	parameters, err := parseArgon2idParameters(parts[3])
	if err != nil {
		return argon2idPasswordHash{}, err
	}

	memory, foundMemory := parameters["m"]
	iterations, foundIterations := parameters["t"]
	parallelism, foundParallelism := parameters["p"]
	if !foundMemory || !foundIterations || !foundParallelism ||
		len(parameters) != 3 {
		return argon2idPasswordHash{}, errors.New(
			"Argon2id hash must define m, t, and p",
		)
	}

	if memory < 64*1024 || memory > 256*1024 {
		return argon2idPasswordHash{}, errors.New(
			"Argon2id memory must be between 65536 and 262144 KiB",
		)
	}
	if iterations < 2 || iterations > 10 {
		return argon2idPasswordHash{}, errors.New(
			"Argon2id iterations must be between 2 and 10",
		)
	}
	if parallelism < 1 || parallelism > 8 {
		return argon2idPasswordHash{}, errors.New(
			"Argon2id parallelism must be between 1 and 8",
		)
	}

	salt, err := decodeRawBase64(parts[4])
	if err != nil || len(salt) < 16 {
		return argon2idPasswordHash{}, errors.New(
			"Argon2id hash has an invalid salt",
		)
	}

	key, err := decodeRawBase64(parts[5])
	if err != nil || len(key) < 32 || len(key) > 64 {
		return argon2idPasswordHash{}, errors.New(
			"Argon2id hash has an invalid key",
		)
	}

	return argon2idPasswordHash{
		memory:      uint32(memory),
		iterations:  uint32(iterations),
		parallelism: uint8(parallelism),
		salt:        salt,
		key:         key,
	}, nil
}

func parseArgon2idParameters(encoded string) (map[string]int, error) {
	parameters := make(map[string]int)

	for _, pair := range strings.Split(encoded, ",") {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, errors.New("Argon2id parameters are malformed")
		}
		if _, exists := parameters[parts[0]]; exists {
			return nil, errors.New("Argon2id parameter is duplicated")
		}

		value, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, errors.New("Argon2id parameter is invalid")
		}
		parameters[parts[0]] = value
	}

	return parameters, nil
}

func decodeRawBase64(encoded string) ([]byte, error) {
	encodings := []*base64.Encoding{
		base64.RawStdEncoding,
		base64.StdEncoding,
		base64.RawURLEncoding,
		base64.URLEncoding,
	}

	for _, encoding := range encodings {
		decoded, err := encoding.DecodeString(encoded)
		if err == nil {
			return decoded, nil
		}
	}

	return nil, errors.New("value is not valid base64")
}

func decodePanelSessionKey(encoded string) ([]byte, error) {
	if encoded == "" {
		return nil, errors.New("value is required")
	}

	key, err := decodeRawBase64(encoded)
	if err != nil || len(key) < 32 {
		return nil, errors.New("value must be base64-encoded and at least 32 bytes")
	}

	return key, nil
}

func newControlPanelFromEnvironment(
	panoptes *PanoptesServer,
) (*controlPanel, error) {
	config, err := loadControlPanelConfig()
	if err != nil {
		return nil, err
	}
	if config == nil {
		return nil, nil
	}

	return newControlPanel(panoptes, *config)
}

func newControlPanel(
	panoptes *PanoptesServer,
	config panelConfig,
) (*controlPanel, error) {
	if panoptes == nil {
		return nil, errors.New("control panel needs a Panoptes server")
	}

	templates, err := template.New("panel").Funcs(template.FuncMap{
		"formatPanelTime": formatPanelTime,
		"panelUnix":       panelUnix,
	}).Parse(panelTemplateSource)
	if err != nil {
		return nil, fmt.Errorf("parse control panel templates: %w", err)
	}

	panel := &controlPanel{
		panoptes:  panoptes,
		address:   config.address,
		auth:      newPanelAuthenticator(config),
		templates: templates,
	}

	panel.http = &http.Server{
		Handler:           panel.withSecurityHeaders(panel.routes()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    8 << 10,
	}

	return panel, nil
}

func newPanelAuthenticator(config panelConfig) *panelAuthenticator {
	return &panelAuthenticator{
		passwordHash:       config.passwordHash,
		sessionKey:         append([]byte(nil), config.sessionKey...),
		sessionLifetime:    config.sessionLifetime,
		sessionIdleTimeout: config.sessionIdleTimeout,
		now:                time.Now,
		sessions:           make(map[string]panelSession),
		failures:           make(map[string]panelLoginFailures),
		verify:             make(chan struct{}, 2),
	}
}

func (panel *controlPanel) listen() (net.Listener, error) {
	tlsConfig, err := panelTLSConfig()
	if err != nil {
		return nil, err
	}

	panel.http.TLSConfig = tlsConfig

	listener, err := net.Listen("tcp", panel.address)
	if err != nil {
		return nil, fmt.Errorf("listen for control panel: %w", err)
	}

	return listener, nil
}

func (panel *controlPanel) serve(listener net.Listener) error {
	return panel.http.ServeTLS(listener, "", "")
}

func (panel *controlPanel) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		http.Redirect(
			writer,
			request,
			panelBasePath,
			http.StatusSeeOther,
		)
	})
	mux.HandleFunc(
		"GET "+panelLoginPath,
		panel.handleLoginPage,
	)
	mux.HandleFunc(
		"POST "+panelLoginPath,
		panel.handleLogin,
	)
	mux.HandleFunc(
		"GET /panel/static/htmx.min.js",
		panel.handleHTMX,
	)
	mux.HandleFunc(
		"GET /panel/static/panel.css",
		panel.handleCSS,
	)
	mux.HandleFunc(
		"GET /panel/static/panel.js",
		panel.handleJavaScript,
	)
	mux.HandleFunc(
		"GET /panel/static/login-decoration.png",
		panel.handleLoginDecoration,
	)

	mux.Handle(
		"POST /panel/logout",
		panel.requireSession(http.HandlerFunc(panel.handleLogout)),
	)
	mux.Handle(
		"GET /panel/fragments/eyes",
		panel.requireSession(http.HandlerFunc(panel.handleEyeRows)),
	)
	mux.Handle(
		"GET /panel/eyes",
		panel.requireSession(http.HandlerFunc(panel.handleEyes)),
	)
	mux.Handle(
		"GET /panel/eyes/{eyeID}",
		panel.requireSession(http.HandlerFunc(panel.handleEyeDetail)),
	)
	mux.Handle(
		"GET /panel/eyes/{eyeID}/status",
		panel.requireSession(http.HandlerFunc(panel.handleEyeStatus)),
	)
	mux.Handle(
		"POST /panel/eyes/{eyeID}/gazes",
		panel.requireSession(http.HandlerFunc(panel.handleSaveGaze)),
	)
	mux.Handle(
		"POST /panel/eyes/{eyeID}/gazes/{sigil}/toggle",
		panel.requireSession(http.HandlerFunc(panel.handleToggleGaze)),
	)
	mux.Handle(
		"GET /panel/oracle-seal",
		panel.requireSession(http.HandlerFunc(panel.handleOracleSealRedirect)),
	)
	mux.Handle(
		"POST /panel/oracle-seal",
		panel.requireSession(http.HandlerFunc(panel.handleForgeOracleSeal)),
	)
	mux.Handle(
		"GET /panel/seals",
		panel.requireSession(http.HandlerFunc(panel.handleSeals)),
	)
	mux.Handle(
		"POST /panel/seals/eye",
		panel.requireSession(http.HandlerFunc(panel.handleForgeEyeSeal)),
	)
	mux.Handle(
		"POST /panel/seals/oracle",
		panel.requireSession(http.HandlerFunc(panel.handleForgeOracleSeal)),
	)
	mux.Handle(
		"GET /panel/oracles",
		panel.requireSession(http.HandlerFunc(panel.handleOracles)),
	)
	mux.Handle(
		"GET /panel/omens",
		panel.requireSession(http.HandlerFunc(panel.handleOmens)),
	)
	mux.Handle(
		"GET "+panelBasePath,
		panel.requireSession(http.HandlerFunc(panel.handleDashboard)),
	)

	return mux
}

func (panel *controlPanel) withSecurityHeaders(
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		writer.Header().Set(
			"Content-Security-Policy",
			"default-src 'self'; base-uri 'none'; form-action 'self'; object-src 'none'; "+
				"frame-ancestors 'none'; script-src 'self'; style-src 'self'; "+
				"img-src 'self' data:; connect-src 'self'",
		)
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set("Referrer-Policy", "same-origin")
		writer.Header().Set(
			"Permissions-Policy",
			"camera=(), geolocation=(), microphone=()",
		)
		writer.Header().Set(
			"Strict-Transport-Security",
			"max-age=31536000",
		)
		writer.Header().Set("Cache-Control", "no-store")

		if request.Method == http.MethodPost ||
			request.Method == http.MethodPut ||
			request.Method == http.MethodPatch {
			request.Body = http.MaxBytesReader(
				writer,
				request.Body,
				panelMaximumBodyBytes,
			)
		}

		next.ServeHTTP(writer, request)
	})
}

func (panel *controlPanel) requireSession(
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(
		writer http.ResponseWriter,
		request *http.Request,
	) {
		cookie, err := request.Cookie(panelCookieName)
		if err != nil {
			panel.handleUnauthenticated(writer, request)
			return
		}

		session, found := panel.auth.lookupSession(cookie.Value)
		if !found {
			panel.handleUnauthenticated(writer, request)
			return
		}

		request = contextWithPanelSession(request, session)
		next.ServeHTTP(writer, request)
	})
}

func contextWithPanelSession(
	request *http.Request,
	session panelSession,
) *http.Request {
	return request.WithContext(context.WithValue(
		request.Context(),
		panelSessionContextKey{},
		session,
	))
}

func panelSessionFromRequest(
	request *http.Request,
) (panelSession, bool) {
	session, found := request.Context().Value(
		panelSessionContextKey{},
	).(panelSession)
	return session, found
}

func (panel *controlPanel) handleUnauthenticated(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if isHTMXRequest(request) {
		writer.Header().Set("HX-Redirect", panelLoginPath)
		http.Error(
			writer,
			"authentication required",
			http.StatusUnauthorized,
		)
		return
	}

	next := request.URL.RequestURI()
	if !strings.HasPrefix(next, panelBasePath) {
		next = panelBasePath
	}

	http.Redirect(
		writer,
		request,
		panelLoginPath+"?next="+url.QueryEscape(next),
		http.StatusSeeOther,
	)
}

func (panel *controlPanel) handleHTMX(
	writer http.ResponseWriter,
	_ *http.Request,
) {
	writer.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	_, _ = writer.Write(panelHTMX)
}

func (panel *controlPanel) handleCSS(
	writer http.ResponseWriter,
	_ *http.Request,
) {
	writer.Header().Set("Content-Type", "text/css; charset=utf-8")
	_, _ = writer.Write(panelCSS)
}

func (panel *controlPanel) handleJavaScript(
	writer http.ResponseWriter,
	_ *http.Request,
) {
	writer.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	_, _ = writer.Write(panelJavaScript)
}

func (panel *controlPanel) handleLoginDecoration(
	writer http.ResponseWriter,
	_ *http.Request,
) {
	writer.Header().Set("Content-Type", "image/png")
	_, _ = writer.Write(panelLoginDecoration)
}

func (panel *controlPanel) handleLoginPage(
	writer http.ResponseWriter,
	request *http.Request,
) {
	panel.render(
		writer,
		"login",
		panelLoginData{
			Next: panelSafeRedirectTarget(request.URL.Query().Get("next")),
		},
		http.StatusOK,
	)
}

func (panel *controlPanel) handleLogin(
	writer http.ResponseWriter,
	request *http.Request,
) {
	if err := request.ParseForm(); err != nil {
		panel.renderLoginFailure(
			writer,
			request,
			http.StatusBadRequest,
		)
		return
	}

	clientAddress := panelClientAddress(request)
	if !panel.auth.loginAllowed(clientAddress) {
		panel.renderLoginFailure(
			writer,
			request,
			http.StatusTooManyRequests,
		)
		return
	}

	password := request.PostForm.Get("password")
	if len(password) > 1024 || !panel.auth.verifyPassword(password) {
		panel.auth.recordLoginFailure(clientAddress)
		panel.renderLoginFailure(
			writer,
			request,
			http.StatusUnauthorized,
		)
		return
	}

	if previousCookie, err := request.Cookie(panelCookieName); err == nil {
		panel.auth.deleteSession(previousCookie.Value)
	}

	sessionID, _, err := panel.auth.createSession()
	if err != nil {
		log.Printf("[PANEL] Could not create admin session: %v", err)
		panel.renderLoginFailure(
			writer,
			request,
			http.StatusInternalServerError,
		)
		return
	}

	panel.auth.recordLoginSuccess(clientAddress)
	http.SetCookie(writer, &http.Cookie{
		Name:     panelCookieName,
		Value:    sessionID,
		Path:     "/",
		MaxAge:   int(panelSessionLifetime / time.Second),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	log.Printf("[PANEL] Administrator authenticated from %s", clientAddress)

	target := panelSafeRedirectTarget(request.PostForm.Get("next"))
	if isHTMXRequest(request) {
		writer.Header().Set("HX-Trigger", "panel:access-granted")
		panel.render(
			writer,
			"login-granted",
			panelLoginData{Next: target},
			http.StatusOK,
		)
		return
	}

	http.Redirect(
		writer,
		request,
		target,
		http.StatusSeeOther,
	)
}

func (panel *controlPanel) renderLoginFailure(
	writer http.ResponseWriter,
	request *http.Request,
	status int,
) {
	data := panelLoginData{
		Error: "ACCESS DENIED",
		Next:  panelSafeRedirectTarget(request.PostForm.Get("next")),
	}

	if isHTMXRequest(request) {
		panel.render(writer, "login-auth", data, http.StatusOK)
		return
	}

	panel.render(writer, "login", data, status)
}

func (panel *controlPanel) handleLogout(
	writer http.ResponseWriter,
	request *http.Request,
) {
	session, valid := panelSessionFromRequest(request)
	if !valid || !panel.validateMutation(writer, request, session) {
		return
	}

	cookie, err := request.Cookie(panelCookieName)
	if err == nil {
		panel.auth.deleteSession(cookie.Value)
	}

	http.SetCookie(writer, &http.Cookie{
		Name:     panelCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	http.Redirect(
		writer,
		request,
		panelLoginPath,
		http.StatusSeeOther,
	)
}

func (panel *controlPanel) handleDashboard(
	writer http.ResponseWriter,
	request *http.Request,
) {
	session, valid := panelSessionFromRequest(request)
	if !valid {
		panel.handleUnauthenticated(writer, request)
		return
	}

	data, err := panel.recallEyesPageData(
		session.csrfToken,
		request.URL.Query().Get("q"),
		request.URL.Query().Get("page"),
		true,
	)
	if err != nil {
		panel.writeInternalError(writer, "recall Eye page", err)
		return
	}

	panel.render(writer, "eyes", data, http.StatusOK)
}

func (panel *controlPanel) handleEyes(
	writer http.ResponseWriter,
	request *http.Request,
) {
	session, valid := panelSessionFromRequest(request)
	if !valid {
		panel.handleUnauthenticated(writer, request)
		return
	}

	data, err := panel.recallEyesPageData(
		session.csrfToken,
		request.URL.Query().Get("q"),
		request.URL.Query().Get("page"),
		true,
	)
	if err != nil {
		panel.writeInternalError(writer, "recall Eye page", err)
		return
	}

	panel.render(writer, "eyes", data, http.StatusOK)
}

func (panel *controlPanel) handleEyeRows(
	writer http.ResponseWriter,
	request *http.Request,
) {
	data, err := panel.recallEyesPageData(
		"",
		request.URL.Query().Get("q"),
		request.URL.Query().Get("page"),
		false,
	)
	if err != nil {
		panel.writeInternalError(writer, "recall Eyes", err)
		return
	}

	panel.render(
		writer,
		"eye-rows",
		data,
		http.StatusOK,
	)
}

func (panel *controlPanel) handleEyeDetail(
	writer http.ResponseWriter,
	request *http.Request,
) {
	session, valid := panelSessionFromRequest(request)
	if !valid {
		panel.handleUnauthenticated(writer, request)
		return
	}

	data, err := panel.recallEyeData(
		request.PathValue("eyeID"),
		session.csrfToken,
	)
	if err != nil {
		panel.writeEyeDataError(writer, err)
		return
	}

	panel.render(writer, "eye-detail", data, http.StatusOK)
}

func (panel *controlPanel) handleEyeStatus(
	writer http.ResponseWriter,
	request *http.Request,
) {
	eye, err := panel.recallEye(request.PathValue("eyeID"))
	if err != nil {
		panel.writeEyeDataError(writer, err)
		return
	}

	panel.render(writer, "eye-status", eye, http.StatusOK)
}

func (panel *controlPanel) handleOracles(
	writer http.ResponseWriter,
	request *http.Request,
) {
	session, valid := panelSessionFromRequest(request)
	if !valid {
		panel.handleUnauthenticated(writer, request)
		return
	}

	data, err := panel.recallOraclesData(session.csrfToken)
	if err != nil {
		panel.writeInternalError(writer, "recall Oracles", err)
		return
	}

	panel.render(writer, "oracles", data, http.StatusOK)
}

func (panel *controlPanel) handleOmens(
	writer http.ResponseWriter,
	request *http.Request,
) {
	session, valid := panelSessionFromRequest(request)
	if !valid {
		panel.handleUnauthenticated(writer, request)
		return
	}

	data, err := panel.recallOmensData(session.csrfToken)
	if err != nil {
		panel.writeInternalError(writer, "recall Omens", err)
		return
	}

	panel.render(writer, "omens", data, http.StatusOK)
}

func (panel *controlPanel) handleSaveGaze(
	writer http.ResponseWriter,
	request *http.Request,
) {
	session, valid := panelSessionFromRequest(request)
	if !valid || !panel.validateMutation(writer, request, session) {
		return
	}

	eyeID := request.PathValue("eyeID")
	if _, err := panel.recallEye(eyeID); err != nil {
		panel.writeEyeDataError(writer, err)
		return
	}

	if err := request.ParseForm(); err != nil {
		panel.renderGazeOutcome(
			writer,
			request,
			eyeID,
			session.csrfToken,
			"",
			"Unable to read the configuration.",
		)
		return
	}

	gaze, err := panelGazeFromForm(eyeID, request.PostForm)
	if err != nil {
		panel.renderGazeOutcome(
			writer,
			request,
			eyeID,
			session.csrfToken,
			"",
			err.Error(),
		)
		return
	}

	updated, err := panel.panoptes.bestowGaze(gaze)
	if err != nil {
		log.Printf(
			"[PANEL] Could not update Gaze %s for Eye %s: %v",
			gaze.Sigil,
			gaze.EyeID,
			err,
		)
		panel.renderGazeOutcome(
			writer,
			request,
			eyeID,
			session.csrfToken,
			"",
			"Unable to save the Gaze configuration.",
		)
		return
	}

	log.Printf(
		"[PANEL] Updated Gaze %s for Eye %s at turn %d",
		updated.Sigil,
		updated.EyeID,
		updated.Turn,
	)
	panel.renderGazeOutcome(
		writer,
		request,
		eyeID,
		session.csrfToken,
		"Gaze configuration saved.",
		"",
	)
}

func (panel *controlPanel) handleToggleGaze(
	writer http.ResponseWriter,
	request *http.Request,
) {
	session, valid := panelSessionFromRequest(request)
	if !valid || !panel.validateMutation(writer, request, session) {
		return
	}

	eyeID := request.PathValue("eyeID")
	if _, err := panel.recallEye(eyeID); err != nil {
		panel.writeEyeDataError(writer, err)
		return
	}

	sigil := strings.TrimSpace(request.PathValue("sigil"))
	if sigil == "" || len(sigil) > 256 {
		panel.renderGazeOutcome(
			writer,
			request,
			eyeID,
			session.csrfToken,
			"",
			"Invalid Gaze.",
		)
		return
	}

	gaze, found, err := panel.panoptes.chronicle.RecallGaze(eyeID, sigil)
	if err != nil {
		panel.writeInternalError(writer, "recall Gaze", err)
		return
	}
	if !found {
		panel.renderGazeOutcome(
			writer,
			request,
			eyeID,
			session.csrfToken,
			"",
			"Gaze no longer exists.",
		)
		return
	}

	gaze.Awake = !gaze.Awake
	updated, err := panel.panoptes.bestowGaze(gaze)
	if err != nil {
		log.Printf(
			"[PANEL] Could not change Gaze %s for Eye %s: %v",
			gaze.Sigil,
			gaze.EyeID,
			err,
		)
		panel.renderGazeOutcome(
			writer,
			request,
			eyeID,
			session.csrfToken,
			"",
			"Unable to change the Gaze state.",
		)
		return
	}

	state := "deactivated"
	if updated.Awake {
		state = "activated"
	}
	log.Printf(
		"[PANEL] %s Gaze %s for Eye %s at turn %d",
		state,
		updated.Sigil,
		updated.EyeID,
		updated.Turn,
	)
	panel.renderGazeOutcome(
		writer,
		request,
		eyeID,
		session.csrfToken,
		"Gaze "+state+".",
		"",
	)
}

func (panel *controlPanel) handleSeals(
	writer http.ResponseWriter,
	request *http.Request,
) {
	session, valid := panelSessionFromRequest(request)
	if !valid {
		panel.handleUnauthenticated(writer, request)
		return
	}

	data, err := panel.newSealsData(
		session.csrfToken,
		panelSealOutcome{},
		panelOracleSealOutcome{},
	)
	if err != nil {
		panel.writeInternalError(writer, "recall panel navigation", err)
		return
	}

	panel.render(writer, "seals", data, http.StatusOK)
}

func (panel *controlPanel) handleOracleSealRedirect(
	writer http.ResponseWriter,
	request *http.Request,
) {
	http.Redirect(writer, request, "/panel/seals", http.StatusSeeOther)
}

func panopticonEndpointForRequest(request *http.Request) (string, error) {
	authority, err := url.Parse(
		"//" + strings.TrimSpace(request.Host),
	)
	if err != nil || authority.User != nil || authority.Host == "" {
		return "", errors.New("panel host is invalid")
	}

	host := authority.Hostname()
	if host == "" || strings.ContainsAny(host, " \t\r\n") {
		return "", errors.New("panel host is invalid")
	}

	return net.JoinHostPort(host, panopticonGRPCPort), nil
}

func (panel *controlPanel) handleForgeEyeSeal(
	writer http.ResponseWriter,
	request *http.Request,
) {
	session, valid := panelSessionFromRequest(request)
	if !valid || !panel.validateMutation(writer, request, session) {
		return
	}

	if err := request.ParseForm(); err != nil ||
		request.PostForm.Get("confirm") != "forge" {
		panel.renderEyeSealOutcome(
			writer,
			request,
			session.csrfToken,
			"",
			time.Time{},
			"Confirmation is required.",
		)
		return
	}

	seal, expiresAt, err := panel.panoptes.issueSeal()
	if err != nil {
		log.Printf("[PANEL] Could not forge Eye Seal: %v", err)
		panel.renderEyeSealOutcome(
			writer,
			request,
			session.csrfToken,
			"",
			time.Time{},
			"Unable to forge an Eye Seal.",
		)
		return
	}

	log.Printf("[PANEL] Forged an Eye Seal expiring at %s", expiresAt.Format(time.RFC3339))
	panel.renderEyeSealOutcome(
		writer,
		request,
		session.csrfToken,
		seal,
		expiresAt,
		"",
	)
}

func (panel *controlPanel) handleForgeOracleSeal(
	writer http.ResponseWriter,
	request *http.Request,
) {
	session, valid := panelSessionFromRequest(request)
	if !valid || !panel.validateMutation(writer, request, session) {
		return
	}

	if err := request.ParseForm(); err != nil ||
		request.PostForm.Get("confirm") != "forge" {
		panel.renderOracleSealOutcome(
			writer,
			request,
			session.csrfToken,
			panelOracleSealOutcome{
				Error: "Confirmation is required.",
			},
		)
		return
	}

	endpoint, err := panopticonEndpointForRequest(request)
	if err != nil {
		panel.renderOracleSealOutcome(
			writer,
			request,
			session.csrfToken,
			panelOracleSealOutcome{
				Error: "Unable to determine the Panopticon endpoint.",
			},
		)
		return
	}

	seal, expiresAt, err := panel.panoptes.issueOracleSeal()
	if err != nil {
		log.Printf("[PANEL] Could not forge Oracle Seal: %v", err)
		panel.renderOracleSealOutcome(
			writer,
			request,
			session.csrfToken,
			panelOracleSealOutcome{
				Endpoint: endpoint,
				Error:    "Unable to forge an Oracle Seal.",
			},
		)
		return
	}

	log.Printf("[PANEL] Forged an Oracle Seal expiring at %s", expiresAt.Format(time.RFC3339))
	panel.renderOracleSealOutcome(
		writer,
		request,
		session.csrfToken,
		panelOracleSealOutcome{
			Endpoint:  endpoint,
			Seal:      seal,
			ExpiresAt: expiresAt,
		},
	)
}

func (panel *controlPanel) renderGazeOutcome(
	writer http.ResponseWriter,
	request *http.Request,
	eyeID string,
	csrfToken string,
	message string,
	errorMessage string,
) {
	data, err := panel.recallEyeData(eyeID, csrfToken)
	if err != nil {
		panel.writeEyeDataError(writer, err)
		return
	}
	data.Message = message
	data.Error = errorMessage

	if isHTMXRequest(request) {
		panel.render(writer, "gaze-panel", data, http.StatusOK)
		return
	}

	if errorMessage != "" {
		panel.render(writer, "eye-detail", data, http.StatusBadRequest)
		return
	}

	http.Redirect(
		writer,
		request,
		"/panel/eyes/"+url.PathEscape(eyeID),
		http.StatusSeeOther,
	)
}

func (panel *controlPanel) renderOracleSealOutcome(
	writer http.ResponseWriter,
	request *http.Request,
	csrfToken string,
	oracle panelOracleSealOutcome,
) {
	data, err := panel.newSealsData(
		csrfToken,
		panelSealOutcome{},
		oracle,
	)
	if err != nil {
		panel.writeInternalError(writer, "recall panel navigation", err)
		return
	}

	if isHTMXRequest(request) {
		panel.render(writer, "oracle-seal-outcome", data, http.StatusOK)
		return
	}

	if oracle.Error != "" {
		panel.render(writer, "seals", data, http.StatusBadRequest)
		return
	}

	panel.render(writer, "seals", data, http.StatusOK)
}

func (panel *controlPanel) renderEyeSealOutcome(
	writer http.ResponseWriter,
	request *http.Request,
	csrfToken string,
	seal string,
	expiresAt time.Time,
	errorMessage string,
) {
	data, err := panel.newSealsData(
		csrfToken,
		panelSealOutcome{
			Seal:      seal,
			ExpiresAt: expiresAt,
			Error:     errorMessage,
		},
		panelOracleSealOutcome{},
	)
	if err != nil {
		panel.writeInternalError(writer, "recall panel navigation", err)
		return
	}

	if isHTMXRequest(request) {
		panel.render(writer, "eye-seal-outcome", data, http.StatusOK)
		return
	}

	if errorMessage != "" {
		panel.render(writer, "seals", data, http.StatusBadRequest)
		return
	}

	panel.render(writer, "seals", data, http.StatusOK)
}

func (panel *controlPanel) validateMutation(
	writer http.ResponseWriter,
	request *http.Request,
	session panelSession,
) bool {
	if hasPanelRequestProvenance(request) && !samePanelOrigin(request) {
		http.Error(writer, "invalid origin", http.StatusForbidden)
		return false
	}

	if err := request.ParseForm(); err != nil {
		http.Error(writer, "invalid request", http.StatusBadRequest)
		return false
	}

	provided := request.PostForm.Get("csrf")
	if provided == "" || subtle.ConstantTimeCompare(
		[]byte(provided),
		[]byte(session.csrfToken),
	) != 1 {
		http.Error(writer, "invalid request", http.StatusForbidden)
		return false
	}

	return true
}

func hasPanelRequestProvenance(request *http.Request) bool {
	return strings.TrimSpace(request.Header.Get("Origin")) != "" ||
		strings.TrimSpace(request.Referer()) != ""
}

func samePanelOrigin(request *http.Request) bool {
	if request.TLS == nil {
		return false
	}

	origin := strings.TrimSpace(request.Header.Get("Origin"))
	if origin != "" {
		return isSamePanelURL(origin, request.Host)
	}

	return isSamePanelURL(request.Referer(), request.Host)
}

func isSamePanelURL(value string, host string) bool {
	parsed, err := url.Parse(value)
	if err != nil ||
		parsed.Scheme != "https" ||
		parsed.Host == "" ||
		parsed.User != nil {
		return false
	}

	return strings.EqualFold(
		strings.TrimSuffix(parsed.Host, "."),
		strings.TrimSuffix(host, "."),
	)
}

func panelGazeFromForm(
	eyeID string,
	values url.Values,
) (GazeRecord, error) {
	sigil := strings.TrimSpace(values.Get("sigil"))
	if sigil == "" {
		sigil = "docker-health"
	}
	if !panelSigilPattern.MatchString(sigil) {
		return GazeRecord{}, errors.New(
			"Sigil must contain 1-64 letters, numbers, dots, dashes, or underscores",
		)
	}

	if values.Get("vision") != panelDockerHealthVision {
		return GazeRecord{}, errors.New("Unsupported Vision configuration")
	}

	target := strings.TrimSpace(values.Get("target"))
	if target == "" || len(target) > 512 || !utf8.ValidString(target) {
		return GazeRecord{}, errors.New(
			"Target must be valid text between 1 and 512 characters",
		)
	}

	reconcileSeconds, err := panelOptionalBoundedInteger(
		values.Get("reconcile_interval_seconds"),
		"Reconcile interval",
		60,
		1,
		86400,
	)
	if err != nil {
		return GazeRecord{}, err
	}

	graceSeconds, err := panelOptionalBoundedInteger(
		values.Get("starting_grace_seconds"),
		"Starting grace",
		120,
		0,
		86400,
	)
	if err != nil {
		return GazeRecord{}, err
	}

	focus, err := structpb.NewStruct(map[string]any{
		"target":                     target,
		"reconcile_interval_seconds": reconcileSeconds,
		"starting_grace_seconds":     graceSeconds,
	})
	if err != nil {
		return GazeRecord{}, errors.New("Unable to construct Gaze configuration")
	}

	return GazeRecord{
		EyeID: eyeID,
		Sigil: sigil,
		Awake: values.Get("awake") == "true",
		Sight: panelDockerHealthVision,
		Form:  panelDockerHealthForm,
		Focus: focus,
	}, nil
}

func panelBoundedInteger(
	value string,
	label string,
	minimum int,
	maximum int,
) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < minimum || parsed > maximum {
		return 0, fmt.Errorf(
			"%s must be between %d and %d",
			label,
			minimum,
			maximum,
		)
	}

	return parsed, nil
}

func panelOptionalBoundedInteger(
	value string,
	label string,
	fallback int,
	minimum int,
	maximum int,
) (int, error) {
	if strings.TrimSpace(value) == "" {
		return fallback, nil
	}

	return panelBoundedInteger(value, label, minimum, maximum)
}

func (panel *controlPanel) recallEyes() ([]panelEye, error) {
	sightings, err := panel.panoptes.chronicle.RecallSightings()
	if err != nil {
		return nil, err
	}

	return panel.eyesFromSightings(sightings)
}

func (panel *controlPanel) eyesFromSightings(
	sightings []Sighting,
) ([]panelEye, error) {
	panel.panoptes.mu.Lock()
	states := make(map[string]EyeState, len(panel.panoptes.eyes))
	for eyeID, state := range panel.panoptes.eyes {
		states[eyeID] = state
	}
	panel.panoptes.mu.Unlock()

	eyes := make([]panelEye, 0, len(sightings))
	for _, sighting := range sightings {
		state := states[sighting.EyeID]
		gazes, err := panel.panoptes.chronicle.RecallGazes(sighting.EyeID)
		if err != nil {
			return nil, err
		}

		sigils := make([]string, 0, len(gazes))
		for _, gaze := range gazes {
			sigils = append(sigils, gaze.Sigil)
		}

		eyes = append(eyes, panelEye{
			ID:        sighting.EyeID,
			FirstSeen: sighting.FirstSeen,
			LastSeen:  sighting.LastSeen,
			Online:    state.Online,
			Sigils:    sigils,
		})
	}

	return eyes, nil
}

func (panel *controlPanel) recallNavigationData(
	csrfToken string,
	currentTab string,
) (panelNavigationData, error) {
	return panelNavigationData{
		CSRF:       csrfToken,
		CurrentTab: currentTab,
	}, nil
}

func (panel *controlPanel) recallEyesPageData(
	csrfToken string,
	query string,
	requestedPage string,
	includeSummary bool,
) (panelEyesData, error) {
	const pageSize = 25

	page, err := panelPageNumber(requestedPage)
	if err != nil {
		return panelEyesData{}, err
	}

	offset := (page - 1) * pageSize
	sightings, total, err := panel.panoptes.chronicle.RecallSightingsPage(
		query,
		pageSize,
		offset,
	)
	if err != nil {
		return panelEyesData{}, err
	}

	totalPages := (total + pageSize - 1) / pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
		offset = (page - 1) * pageSize
		sightings, total, err = panel.panoptes.chronicle.RecallSightingsPage(
			query,
			pageSize,
			offset,
		)
		if err != nil {
			return panelEyesData{}, err
		}
	}

	eyes, err := panel.eyesFromSightings(sightings)
	if err != nil {
		return panelEyesData{}, err
	}

	var summary panelEyeSummary
	if includeSummary {
		summary, err = panel.recallEyeSummary(query)
		if err != nil {
			return panelEyesData{}, err
		}
	}

	navigation, err := panel.recallNavigationData(csrfToken, "eyes")
	if err != nil {
		return panelEyesData{}, err
	}

	return panelEyesData{
		panelNavigationData: navigation,
		Eyes:                eyes,
		Query:               strings.TrimSpace(query),
		Summary:             summary,
		Pagination: panelPagination{
			Page:         page,
			TotalPages:   totalPages,
			Total:        total,
			HasPrevious:  page > 1,
			HasNext:      page < totalPages,
			PreviousPage: page - 1,
			NextPage:     page + 1,
		},
	}, nil
}

func (panel *controlPanel) recallEyeSummary(
	query string,
) (panelEyeSummary, error) {
	eyes, err := panel.recallEyes()
	if err != nil {
		return panelEyeSummary{}, err
	}

	needle := strings.ToLower(strings.TrimSpace(query))
	sigils := make(map[string]struct{})
	summary := panelEyeSummary{}

	for _, eye := range eyes {
		if needle != "" &&
			!strings.Contains(strings.ToLower(eye.ID), needle) {
			continue
		}

		summary.All++
		if eye.Online {
			summary.Open++
		} else {
			summary.Closed++
		}

		for _, sigil := range eye.Sigils {
			sigils[sigil] = struct{}{}
		}
	}

	summary.SigilCount = len(sigils)

	return summary, nil
}

func panelPageNumber(value string) (int, error) {
	if strings.TrimSpace(value) == "" {
		return 1, nil
	}

	page, err := strconv.Atoi(value)
	if err != nil || page < 1 || page > 100000 {
		return 0, errors.New("page number is invalid")
	}

	return page, nil
}

func (panel *controlPanel) recallOraclesData(
	csrfToken string,
) (panelOraclesData, error) {
	navigation, err := panel.recallNavigationData(csrfToken, "oracles")
	if err != nil {
		return panelOraclesData{}, err
	}

	records, err := panel.panoptes.chronicle.RecallOracles()
	if err != nil {
		return panelOraclesData{}, err
	}

	oracles := make([]panelOracle, 0, len(records))
	for _, record := range records {
		oracles = append(oracles, panelOracle{
			ID:        record.OracleID,
			PairedAt:  record.PairedAt,
			RevokedAt: record.RevokedAt,
		})
	}

	return panelOraclesData{
		panelNavigationData: navigation,
		Oracles:             oracles,
	}, nil
}

func (panel *controlPanel) recallOmensData(
	csrfToken string,
) (panelOmensData, error) {
	navigation, err := panel.recallNavigationData(csrfToken, "omens")
	if err != nil {
		return panelOmensData{}, err
	}

	records, err := panel.panoptes.chronicle.RecallRecentOmens(50)
	if err != nil {
		return panelOmensData{}, err
	}

	return panelOmensData{
		panelNavigationData: navigation,
		Omens:               panelOmensFromRecords(records),
	}, nil
}

func (panel *controlPanel) newSealsData(
	csrfToken string,
	eye panelSealOutcome,
	oracle panelOracleSealOutcome,
) (panelSealsData, error) {
	navigation, err := panel.recallNavigationData(csrfToken, "seals")
	if err != nil {
		return panelSealsData{}, err
	}

	records, err := panel.panoptes.chronicle.RecallSeals()
	if err != nil {
		return panelSealsData{}, err
	}

	data := panelSealsData{
		panelNavigationData: navigation,
		Eye:                 eye,
		Oracle:              oracle,
		SealHistory:         panelSealHistoryFromRecords(records, time.Now().UTC()),
	}

	if oracle.Seal == "" {
		return data, nil
	}

	qrCodeDataURL, err := oraclePairingQRCode(
		oracle.Endpoint,
		oracle.Seal,
		oracle.ExpiresAt,
	)
	if err != nil {
		return panelSealsData{}, err
	}
	data.Oracle.QRCodeDataURL = qrCodeDataURL

	return data, nil
}

func panelSealHistoryFromRecords(
	records []SealRecord,
	now time.Time,
) []panelSealHistoryItem {
	history := make([]panelSealHistoryItem, 0, len(records))
	for _, record := range records {
		item := panelSealHistoryItem{
			Kind:         record.Kind,
			ForgedAt:     record.ForgedAt,
			ExpiresAt:    record.ExpiresAt,
			Availability: "Available",
		}

		if record.ConsumedAt != nil {
			item.Availability = "Consumed"
			item.Consumed = true
			item.ConsumedAt = *record.ConsumedAt
		} else if now.After(record.ExpiresAt) {
			item.Availability = "Expired"
		}

		history = append(history, item)
	}

	return history
}

func oraclePairingQRCode(
	endpoint string,
	oracleSeal string,
	expiresAt time.Time,
) (template.URL, error) {
	payload, err := oraclePairingPayload(endpoint, oracleSeal, expiresAt)
	if err != nil {
		return "", err
	}

	code, err := qrcode.New(string(payload), qrcode.Medium)
	if err != nil {
		return "", fmt.Errorf("create Oracle pairing QR code: %w", err)
	}
	code.DisableBorder = true

	png, err := code.PNG(320)
	if err != nil {
		return "", fmt.Errorf("encode Oracle pairing QR code: %w", err)
	}

	// The URL is generated exclusively from our PNG bytes and is safe to place
	// in the authenticated, no-store pairing response.
	return template.URL(
		"data:image/png;base64," + base64.StdEncoding.EncodeToString(png),
	), nil
}

func oraclePairingPayload(
	endpoint string,
	oracleSeal string,
	expiresAt time.Time,
) ([]byte, error) {
	payload, err := json.Marshal(oraclePairingQRPayload{
		Schema:        oraclePairingQRSchema,
		Version:       oraclePairingQRVersion,
		Endpoint:      endpoint,
		OracleSeal:    oracleSeal,
		ExpiresAtUnix: expiresAt.Unix(),
	})
	if err != nil {
		return nil, fmt.Errorf("encode Oracle pairing payload: %w", err)
	}

	return payload, nil
}

func panelOmensFromRecords(records []OmenRecord) []panelOmen {
	omens := make([]panelOmen, 0, len(records))
	for _, record := range records {
		omens = append(omens, panelOmen{
			EyeID:      record.EyeID,
			GazeSigil:  record.GazeSigil,
			GazeTurn:   record.GazeTurn,
			BefallenAt: record.BefallenAt,
			ReceivedAt: record.ReceivedAt,
		})
	}

	return omens
}

func (panel *controlPanel) recallEye(eyeID string) (panelEye, error) {
	eyeID = strings.TrimSpace(eyeID)
	if eyeID == "" || len(eyeID) > 256 {
		return panelEye{}, errPanelEyeNotFound
	}

	eyes, err := panel.recallEyes()
	if err != nil {
		return panelEye{}, err
	}

	eye, found := panelEyeWithID(eyes, eyeID)
	if found {
		return eye, nil
	}

	return panelEye{}, errPanelEyeNotFound
}

func panelEyeWithID(
	eyes []panelEye,
	eyeID string,
) (panelEye, bool) {
	for _, eye := range eyes {
		if eye.ID == eyeID {
			return eye, true
		}
	}

	return panelEye{}, false
}

func (panel *controlPanel) recallEyeData(
	eyeID string,
	csrfToken string,
) (panelEyeData, error) {
	navigation, err := panel.recallNavigationData(csrfToken, "eyes")
	if err != nil {
		return panelEyeData{}, err
	}

	eye, err := panel.recallEye(eyeID)
	if err != nil {
		return panelEyeData{}, err
	}

	visions, err := panel.panoptes.chronicle.RecallVisions(eye.ID)
	if err != nil {
		return panelEyeData{}, err
	}

	gazeRecords, err := panel.panoptes.chronicle.RecallGazes(eye.ID)
	if err != nil {
		return panelEyeData{}, err
	}

	gazes := make([]panelGaze, 0, len(gazeRecords))
	for _, gaze := range gazeRecords {
		gazes = append(gazes, panelGazeFromRecord(gaze))
	}

	configurableDockerHealth := false
	for _, vision := range visions {
		if vision.Sight == panelDockerHealthVision &&
			vision.Form == panelDockerHealthForm {
			configurableDockerHealth = true
			break
		}
	}

	return panelEyeData{
		panelNavigationData:      navigation,
		Eye:                      eye,
		Visions:                  visions,
		ConfigurableDockerHealth: configurableDockerHealth,
		Gazes:                    gazes,
	}, nil
}

func panelGazeFromRecord(record GazeRecord) panelGaze {
	gaze := panelGaze{
		Sigil:  record.Sigil,
		Turn:   record.Turn,
		Awake:  record.Awake,
		Vision: record.Sight,
		Form:   record.Form,
	}

	if record.Focus == nil {
		return gaze
	}

	fields := record.Focus.GetFields()
	gaze.Target = strings.TrimSpace(fields["target"].GetStringValue())
	gaze.ReconcileIntervalSeconds = panelFocusNumber(
		fields["reconcile_interval_seconds"],
	)
	gaze.StartingGraceSeconds = panelFocusNumber(
		fields["starting_grace_seconds"],
	)

	return gaze
}

func panelFocusNumber(value *structpb.Value) string {
	if value == nil {
		return ""
	}

	number, ok := value.Kind.(*structpb.Value_NumberValue)
	if !ok {
		return ""
	}

	return strconv.FormatFloat(number.NumberValue, 'f', -1, 64)
}

func (panel *controlPanel) writeEyeDataError(
	writer http.ResponseWriter,
	err error,
) {
	if errors.Is(err, errPanelEyeNotFound) {
		http.Error(writer, "Eye not found", http.StatusNotFound)
		return
	}

	panel.writeInternalError(writer, "recall Eye data", err)
}

func (panel *controlPanel) writeInternalError(
	writer http.ResponseWriter,
	operation string,
	err error,
) {
	log.Printf("[PANEL] Could not %s: %v", operation, err)
	http.Error(
		writer,
		"Panopticon could not complete that request",
		http.StatusInternalServerError,
	)
}

func (panel *controlPanel) render(
	writer http.ResponseWriter,
	name string,
	data any,
	status int,
) {
	writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	writer.WriteHeader(status)

	if err := panel.templates.ExecuteTemplate(writer, name, data); err != nil {
		log.Printf("[PANEL] Render %s: %v", name, err)
	}
}

func (auth *panelAuthenticator) verifyPassword(password string) bool {
	select {
	case auth.verify <- struct{}{}:
		defer func() {
			<-auth.verify
		}()
	default:
		return false
	}

	derivedKey := argon2.IDKey(
		[]byte(password),
		auth.passwordHash.salt,
		auth.passwordHash.iterations,
		auth.passwordHash.memory,
		auth.passwordHash.parallelism,
		uint32(len(auth.passwordHash.key)),
	)

	return subtle.ConstantTimeCompare(
		derivedKey,
		auth.passwordHash.key,
	) == 1
}

func (auth *panelAuthenticator) createSession() (
	string,
	string,
	error,
) {
	rawID := make([]byte, 32)
	if _, err := rand.Read(rawID); err != nil {
		return "", "", fmt.Errorf("generate session ID: %w", err)
	}

	rawCSRF := make([]byte, 32)
	if _, err := rand.Read(rawCSRF); err != nil {
		return "", "", fmt.Errorf("generate CSRF token: %w", err)
	}

	sessionID := base64.RawURLEncoding.EncodeToString(rawID)
	csrfToken := base64.RawURLEncoding.EncodeToString(rawCSRF)
	storedID := auth.storedSessionID(rawID)
	now := auth.now().UTC()

	auth.mu.Lock()
	defer auth.mu.Unlock()

	auth.pruneLocked(now)
	if len(auth.sessions) >= panelMaximumSessions {
		return "", "", errors.New("too many active sessions")
	}

	auth.sessions[storedID] = panelSession{
		csrfToken: csrfToken,
		createdAt: now,
		lastSeen:  now,
	}

	return sessionID, csrfToken, nil
}

func (auth *panelAuthenticator) lookupSession(
	rawSessionID string,
) (panelSession, bool) {
	storedID, valid := auth.storedIDFromCookie(rawSessionID)
	if !valid {
		return panelSession{}, false
	}

	now := auth.now().UTC()

	auth.mu.Lock()
	defer auth.mu.Unlock()

	auth.pruneLocked(now)
	session, found := auth.sessions[storedID]
	if !found {
		return panelSession{}, false
	}

	if now.Sub(session.createdAt) > auth.sessionLifetime ||
		now.Sub(session.lastSeen) > auth.sessionIdleTimeout {
		delete(auth.sessions, storedID)
		return panelSession{}, false
	}

	session.lastSeen = now
	auth.sessions[storedID] = session

	return session, true
}

func (auth *panelAuthenticator) deleteSession(rawSessionID string) {
	storedID, valid := auth.storedIDFromCookie(rawSessionID)
	if !valid {
		return
	}

	auth.mu.Lock()
	delete(auth.sessions, storedID)
	auth.mu.Unlock()
}

func (auth *panelAuthenticator) storedIDFromCookie(
	rawSessionID string,
) (string, bool) {
	rawID, err := base64.RawURLEncoding.DecodeString(rawSessionID)
	if err != nil || len(rawID) != 32 {
		return "", false
	}

	return auth.storedSessionID(rawID), true
}

func (auth *panelAuthenticator) storedSessionID(rawID []byte) string {
	mac := hmac.New(sha256.New, auth.sessionKey)
	_, _ = mac.Write(rawID)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (auth *panelAuthenticator) loginAllowed(clientAddress string) bool {
	now := auth.now().UTC()

	auth.mu.Lock()
	defer auth.mu.Unlock()

	auth.pruneLocked(now)
	failures, found := auth.failures[clientAddress]
	if !found {
		return true
	}

	return !now.Before(failures.blockedUntil)
}

func (auth *panelAuthenticator) recordLoginFailure(clientAddress string) {
	now := auth.now().UTC()

	auth.mu.Lock()
	defer auth.mu.Unlock()

	auth.pruneLocked(now)
	failures := auth.failures[clientAddress]
	if failures.windowOpened.IsZero() ||
		now.Sub(failures.windowOpened) > panelLoginFailureAge {
		failures = panelLoginFailures{windowOpened: now}
	}

	failures.count++
	failures.lastSeen = now
	if failures.count >= panelLoginFailureLimit {
		failures.blockedUntil = now.Add(panelLoginFailureAge)
	}

	if _, found := auth.failures[clientAddress]; !found &&
		len(auth.failures) >= panelMaximumLoginAddresses {
		for address := range auth.failures {
			delete(auth.failures, address)
			break
		}
	}
	auth.failures[clientAddress] = failures
}

func (auth *panelAuthenticator) recordLoginSuccess(clientAddress string) {
	auth.mu.Lock()
	delete(auth.failures, clientAddress)
	auth.mu.Unlock()
}

func (auth *panelAuthenticator) pruneLocked(now time.Time) {
	for sessionID, session := range auth.sessions {
		if now.Sub(session.createdAt) > auth.sessionLifetime ||
			now.Sub(session.lastSeen) > auth.sessionIdleTimeout {
			delete(auth.sessions, sessionID)
		}
	}

	for clientAddress, failures := range auth.failures {
		if now.Sub(failures.lastSeen) > panelLoginFailureAge {
			delete(auth.failures, clientAddress)
		}
	}
}

func panelSafeRedirectTarget(value string) string {
	if strings.HasPrefix(value, panelBasePath) &&
		!strings.HasPrefix(value, "//") {
		return value
	}

	return panelBasePath
}

func panelClientAddress(request *http.Request) string {
	host, _, err := net.SplitHostPort(request.RemoteAddr)
	if err == nil && host != "" {
		return host
	}

	if request.RemoteAddr != "" {
		return request.RemoteAddr
	}

	return "unknown"
}

func isHTMXRequest(request *http.Request) bool {
	return request.Header.Get("HX-Request") == "true"
}

func formatPanelTime(value time.Time) string {
	if value.IsZero() {
		return "never"
	}

	return value.UTC().Format("2006-01-02 15:04:05 UTC")
}

func panelUnix(value time.Time) int64 {
	if value.IsZero() {
		return 0
	}

	return value.Unix()
}
