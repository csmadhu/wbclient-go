package wbclientgo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"

	"github.com/gorilla/mux"
	"github.com/madhucs/wbclient-go/log"
)

const (
	defaultDomainJoinScript  = "/usr/src/scripts/domain-join.sh"
	defaultDomainLeaveScript = "/usr/src/scripts/domain-leave.sh"

	defaultMachinePasswordTimeoutDays = 30
	secondsPerDay                     = 86400
)

const (
	UserValidate = "samba.user.validate"
	UserAuth     = "samba.user.auth"
	DomainJoin   = "samba.domain.join"
	DomainLeave  = "samba.domain.leave"
)

func initRoutes(router *mux.Router) {
	router.HandleFunc(fmt.Sprintf("/%s", UserValidate), createApiHandler(apiUserValidate)).Methods("POST")
	router.HandleFunc(fmt.Sprintf("/%s", UserAuth), createApiHandler(apiUserAuth)).Methods("POST")
	router.HandleFunc(fmt.Sprintf("/%s", DomainJoin), createApiHandler(apiDomainJoin)).Methods("POST")
	router.HandleFunc(fmt.Sprintf("/%s", DomainLeave), createApiHandler(apiDomainLeave)).Methods("POST")
}

func createApiHandler(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		logLabels := fetchLogLabelsFromRequest(r)
		if len(logLabels) != 0 {
			ctx = log.Context(ctx, logLabels...)
		}

		log.WithCtx(ctx).Printf("wbclient - process url[%s]", r.URL)

		// get authorization token
		token := r.Header.Get("Authorization")
		if apiToken := os.Getenv("WBCLIENT_API_TOKEN"); apiToken != "" && apiToken != token {
			w.WriteHeader(http.StatusUnauthorized)
			log.WithCtx(ctx).Printf("wbclient - url[%s] unauthorized", r.URL)
			return
		}

		fn(w, r)
		log.WithCtx(ctx).Printf("wbclient - url[%s] completed", r.URL)
	}
}

// Example: "X_WBCLIENT_REQ_META: {"request_id":"dummy-request-123","correlation_id":"dummy-corr-456", "session_id":"dummy-session-789","client_version":"1.0.0","user_agent":"curl-test"}"
// Return []string{"key1", "value1", "key2", "value2", ...}
func fetchLogLabelsFromRequest(r *http.Request) []string {
	metadataHeader := r.Header.Get("X_WBCLIENT_REQ_META")
	if metadataHeader == "" {
		return nil
	}

	metadata := map[string]string{}
	if err := json.Unmarshal([]byte(metadataHeader), &metadata); err != nil {
		return nil
	}

	var logLabels []string
	for key, value := range metadata {
		logLabels = append(logLabels, key, value)
	}

	return logLabels
}

// decodeReq is a helper function to decode JSON request body
func decodeReq(body io.ReadCloser, v interface{}) error {
	defer body.Close()
	return json.NewDecoder(body).Decode(v)
}

// apiUserValidate handles user authentication with password
// Uses either plain text auth or MSCHAPv2 based on IsPlainTextAuth flag
func apiUserValidate(w http.ResponseWriter, r *http.Request) { //api rename
	var req UserValidateReq
	ctx := r.Context()

	if err := decodeReq(r.Body, &req); err != nil {
		log.WithCtx(ctx).Errorf("wbclient(usertest) - decode request err=%v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Username == "" || req.Password == "" || req.Domain == "" {
		log.WithCtx(ctx).Errorf("wbclient(usertest) - missing required fields")
		w.WriteHeader(http.StatusBadRequest)
		resp := UserAuthResp{
			Success:      false,
			ErrorCode:    -1,
			ErrorMessage: "Username, password, and domain are required",
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	var authResult UserAuthResp

	// Call appropriate authentication helper based on IsPlainTextAuth flag
	if req.IsPlainTextAuth {
		// Plain text authentication
		authResult = AuthenticateWithPlainText(ctx, req)
	} else {
		// MSCHAPv2 authentication with password
		authResult = AuthenticateWithChallenge(ctx, req)
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(authResult); err != nil {
		log.WithCtx(ctx).Errorf("wbclient(usertest) - send resp err=%v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// apiUserAuth handles MSCHAPv2 authentication
func apiUserAuth(w http.ResponseWriter, r *http.Request) {
	var userAuthReq UserAuthReq
	ctx := r.Context()

	if err := decodeReq(r.Body, &userAuthReq); err != nil {
		log.WithCtx(ctx).Errorf("wbclient(userauth) - decode request err=%v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Call AuthenticateMSCHAPv2 function
	resp := AuthenticateMSCHAPv2(ctx, userAuthReq)

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.WithCtx(ctx).Errorf("wbclient(userauth) - send resp err=%v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func apiDomainJoin(w http.ResponseWriter, r *http.Request) {
	var req DomainJoinReq
	ctx := r.Context()

	if err := decodeReq(r.Body, &req); err != nil {
		log.WithCtx(ctx).Errorf("wbclient(domainjoin) - decode request err=%v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if req.DCFQDN == "" || req.NetbiosName == "" || req.ADUsername == "" || req.ADPassword == "" {
		log.WithCtx(ctx).Errorf("wbclient(domainjoin) - missing required fields")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(DomainOpsResp{
			ErrorMessage: "dcfqdn, netbiosName, adUsername and adPassword are all required",
		})
		return
	}

	log.WithCtx(ctx).Printf("wbclient(domainjoin) - request: dcfqdn[%s] netbios[%s] user[%s]",
		req.DCFQDN, req.NetbiosName, req.ADUsername)

	if err := exec.CommandContext(ctx, "net", "ads", "testjoin").Run(); err == nil {
		log.WithCtx(ctx).Printf("wbclient(domainjoin) - already joined; skipping script")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(DomainOpsResp{
			Success:      true,
			ErrorMessage: "already joined",
		})
		return
	}

	timeoutDays := req.PasswordTimeout
	if timeoutDays <= 0 {
		timeoutDays = defaultMachinePasswordTimeoutDays
	}
	timeoutSeconds := timeoutDays * secondsPerDay

	cmd := exec.CommandContext(ctx, "bash", defaultDomainJoinScript)
	cmd.Env = append(os.Environ(),
		"DC_FQDN="+req.DCFQDN,
		"NETBIOS_NAME="+req.NetbiosName,
		"AD_USERNAME="+req.ADUsername,
		"AD_PASSWORD="+req.ADPassword,
		fmt.Sprintf("MACHINE_PASSWORD_TIMEOUT=%d", timeoutSeconds),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.WithCtx(ctx).Errorf("wbclient(domainjoin) - script %s failed err=%v output=%s",
			defaultDomainJoinScript, err, string(output))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(DomainOpsResp{
			ErrorMessage: fmt.Sprintf("domain-join script failed: %v", err),
		})
		return
	}

	log.WithCtx(ctx).Printf("wbclient(domainjoin) - script succeeded")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(DomainOpsResp{Success: true})
}

func apiDomainLeave(w http.ResponseWriter, r *http.Request) {
	var req DomainLeaveReq
	ctx := r.Context()
	if err := decodeReq(r.Body, &req); err != nil {
		log.WithCtx(ctx).Errorf("wbclient(domainleave) - decode request err=%v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if req.ADUsername == "" || req.ADPassword == "" {
		log.WithCtx(ctx).Errorf("wbclient(domainleave) - missing required fields")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(DomainOpsResp{
			ErrorMessage: "adUsername and adPassword are required",
		})
		return
	}

	log.WithCtx(ctx).Printf("wbclient(domainleave) - request: domain[%s] user[%s]",
		req.Domain, req.ADUsername)

	if err := exec.CommandContext(ctx, "net", "ads", "testjoin").Run(); err != nil {
		log.WithCtx(ctx).Printf("wbclient(domainleave) - not currently joined; nothing to do")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(DomainOpsResp{
			Success:      true,
			ErrorMessage: "not joined",
		})
		return
	}

	cmd := exec.CommandContext(ctx, "bash", defaultDomainLeaveScript)
	cmd.Env = append(os.Environ(),
		"AD_USERNAME="+req.ADUsername,
		"AD_PASSWORD="+req.ADPassword,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.WithCtx(ctx).Errorf("wbclient(domainleave) - script %s failed err=%v output=%s",
			defaultDomainLeaveScript, err, string(output))
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(DomainOpsResp{
			ErrorMessage: fmt.Sprintf("domain-leave script failed: %v", err),
		})
		return
	}

	log.WithCtx(ctx).Printf("wbclient(domainleave) - script succeeded")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(DomainOpsResp{Success: true})
}
