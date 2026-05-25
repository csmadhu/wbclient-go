package wbclientgo

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"

	"github.com/gorilla/mux"
	"github.com/madhucs/wbclient-go/log"
)

func Start() {
	validateConfig()
	restoreWinbindd()
	router := mux.NewRouter()
	initRoutes(router)
	startServer(router)
}

func restoreWinbindd() {
	ctx := context.Background()
	cmd := exec.Command("/usr/sbin/winbindd", "-D")
	if err := cmd.Run(); err != nil {
		log.WithCtx(ctx).Printf("wbclient - winbindd not started at boot: %v", err)
		return
	}
	log.WithCtx(ctx).Printf("wbclient - winbindd restored from persisted join state")
}

func validateConfig() {
	ctx := context.Background()
	requiredEnvList := []string{
		"WBCLIENT_API_TOKEN",
		"WBCLIENT_PORT",
	}

	for _, env := range requiredEnvList {
		if os.Getenv(env) == "" {
			log.WithCtx(ctx).Fatalf("wbclient - env %q not set", env)
		}
	}
}

func startServer(router *mux.Router) {
	ctx := context.Background()
	// start http server
	httpServer := &http.Server{
		Handler: router,
	}

	port := os.Getenv("WBCLIENT_PORT")
	if port == "" {
		port = "8080"
	}
	l, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%s", port))
	if err != nil {
		log.WithCtx(ctx).Fatalf("wbclient - failed to listen on port %s: %v", port, err)
	}

	log.WithCtx(ctx).Printf("wbclient - started at %s", port)
	log.WithCtx(ctx).Printf("wbclient - stopped err: %v", httpServer.Serve(l))
}
