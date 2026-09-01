package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"

	wbclientgo "github.com/csmadhu/wbclient-go"
	"github.com/csmadhu/wbclient-go/log"
)

var (
	authRequestTimeout      = envDuration("WBCLIENT_AUTH_TIMEOUT", 10*time.Second)
	domainOpsRequestTimeout = envDuration("WBCLIENT_DOMAIN_OPS_TIMEOUT", 300*time.Second)
)

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func initRoutes(router *mux.Router) {
	router.HandleFunc(fmt.Sprintf("/%s", wbclientgo.UserValidate), createApiHandler(apiUserValidate, authRequestTimeout)).Methods("POST")
	router.HandleFunc(fmt.Sprintf("/%s", wbclientgo.UserAuth), createApiHandler(apiUserAuth, authRequestTimeout)).Methods("POST")
	router.HandleFunc(fmt.Sprintf("/%s", wbclientgo.DomainJoin), createApiHandler(apiDomainJoin, domainOpsRequestTimeout)).Methods("POST")
	router.HandleFunc(fmt.Sprintf("/%s", wbclientgo.DomainLeave), createApiHandler(apiDomainLeave, domainOpsRequestTimeout)).Methods("POST")
	router.HandleFunc(fmt.Sprintf("/%s", wbclientgo.DomainJoinStatus),
		createApiHandler(apiDomainJoinStatus, domainOpsRequestTimeout)).Methods("POST")
	router.HandleFunc(fmt.Sprintf("/%s", wbclientgo.LogLevel), createApiHandler(apiSetLogLevel, domainOpsRequestTimeout)).Methods("POST")
}

func createApiHandler(fn http.HandlerFunc, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		logLabels := fetchLogLabelsFromRequest(r)
		if len(logLabels) != 0 {
			ctx = log.Context(ctx, logLabels...)
		}

		log.WithCtx(ctx).Printf("wbclient - process url[%s]", r.URL)

		token := r.Header.Get("Authorization")
		if apiToken := os.Getenv("WBCLIENT_API_TOKEN"); apiToken != "" && apiToken != token {
			w.WriteHeader(http.StatusUnauthorized)
			log.WithCtx(ctx).Printf("wbclient - url[%s] unauthorized", r.URL)
			return
		}

		r = r.WithContext(ctx)
		fn(w, r)
		log.WithCtx(ctx).Printf("wbclient - url[%s] completed", r.URL)
	}
}

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

func decodeReq(body io.ReadCloser, v any) error {
	defer body.Close()
	return json.NewDecoder(body).Decode(v)
}

func apiUserValidate(w http.ResponseWriter, r *http.Request) {
	var req wbclientgo.UserValidateReq
	ctx := r.Context()

	if err := decodeReq(r.Body, &req); err != nil {
		log.WithCtx(ctx).Errorf("wbclient(usertest) - decode request err=%v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" || req.Netbios == "" {
		log.WithCtx(ctx).Errorf("wbclient(usertest) - missing required fields")
		w.WriteHeader(http.StatusBadRequest)
		resp := wbclientgo.UserAuthResp{
			Success:      false,
			ErrorCode:    -1,
			ErrorMessage: "Username, password, and domain are required",
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	var authResult wbclientgo.UserAuthResp

	if req.IsPlainTextAuth {
		authResult = AuthenticateWithPlainText(ctx, req)
	} else {
		authResult = AuthenticateWithChallenge(ctx, req)
	}

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(authResult); err != nil {
		log.WithCtx(ctx).Errorf("wbclient(usertest) - send resp err=%v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func apiUserAuth(w http.ResponseWriter, r *http.Request) {
	var userAuthReq wbclientgo.UserAuthReq
	ctx := r.Context()

	if err := decodeReq(r.Body, &userAuthReq); err != nil {
		log.WithCtx(ctx).Errorf("wbclient(userauth) - decode request err=%v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	resp := AuthenticateMSCHAPv2(ctx, userAuthReq)

	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.WithCtx(ctx).Errorf("wbclient(userauth) - send resp err=%v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func apiDomainJoin(w http.ResponseWriter, r *http.Request) {
	var req wbclientgo.DomainJoinReq
	ctx := r.Context()

	if err := decodeReq(r.Body, &req); err != nil {
		log.WithCtx(ctx).Errorf("wbclient(domainjoin) - decode request err=%v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if req.DCFQDN == "" || req.NetbiosName == "" || req.ADUsername == "" || req.ADPassword == "" {
		log.WithCtx(ctx).Errorf("wbclient(domainjoin) - missing required fields")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(wbclientgo.DomainOpsResp{
			ErrorMessage: "dcfqdn, netbiosName, adUsername and adPassword are all required",
		})
		return
	}

	resp := DomainJoin(ctx, req)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func apiDomainLeave(w http.ResponseWriter, r *http.Request) {
	var req wbclientgo.DomainLeaveReq
	ctx := r.Context()

	if err := decodeReq(r.Body, &req); err != nil {
		log.WithCtx(ctx).Errorf("wbclient(domainleave) - decode request err=%v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if req.ADUsername == "" || req.ADPassword == "" {
		log.WithCtx(ctx).Errorf("wbclient(domainleave) - missing required fields")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(wbclientgo.DomainOpsResp{
			ErrorMessage: "adUsername and adPassword are required",
		})
		return
	}

	resp := DomainLeave(ctx, req)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func apiSetLogLevel(w http.ResponseWriter, r *http.Request) {
	var req wbclientgo.SetLogLevelReq
	ctx := r.Context()

	if err := decodeReq(r.Body, &req); err != nil {
		log.WithCtx(ctx).Errorf("wbclient(setloglevel) - decode request err=%v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if req.LogLevel < 0 || req.LogLevel > 10 {
		log.WithCtx(ctx).Errorf("wbclient(setloglevel) - invalid log level: %d", req.LogLevel)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(wbclientgo.DomainOpsResp{
			ErrorMessage: "logLevel must be between 0 and 10",
		})
		return
	}

	resp := SetLogLevel(ctx, req)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}

func apiDomainJoinStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	resp := DomainJoinStatus(ctx)
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
