package internal

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	wbclientgo "github.com/csmadhu/wbclient-go"
	"github.com/csmadhu/wbclient-go/log"
)

const (
	defaultDomainJoinScript       = "/usr/src/wbclient/scripts/domain-join.sh"
	defaultDomainLeaveScript      = "/usr/src/wbclient/scripts/domain-leave.sh"
	defaultDomainJoinStatusScript = "/usr/src/wbclient/scripts/domain-join-status.sh"
	defaultSetLogLevelScript      = "/usr/src/wbclient/scripts/set-log-level.sh"
	defaultMigrateConfigScript   = "/usr/src/wbclient/scripts/migrate-config.sh"

	defaultMachinePasswordTimeoutDays = 30
	secondsPerDay                     = 86400
)

func scriptError(stderr string, err error) string {
	if msg := strings.TrimSpace(stderr); msg != "" {
		return msg
	}
	return fmt.Sprintf("UNKNOWN_ERROR: %v", err)
}

func runScript(ctx context.Context, script string, env []string, label string) wbclientgo.DomainOpsResp {
	cmd := exec.CommandContext(ctx, "bash", script)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	stdoutPipe, _ := cmd.StdoutPipe()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		log.WithCtx(ctx).Errorf("wbclient(%s) - script start failed err=%v", label, err)
		return wbclientgo.DomainOpsResp{
			ErrorMessage: fmt.Sprintf("UNKNOWN_ERROR: %v", err),
		}
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			log.WithCtx(ctx).Printf("wbclient(%s) - %s", label, scanner.Text())
		}
	}()

	wg.Wait()
	err := cmd.Wait()

	if err != nil {
		log.WithCtx(ctx).Errorf("wbclient(%s) - script failed err=%v stderr=%s", label, err, stderr.String())
		return wbclientgo.DomainOpsResp{
			ErrorMessage: scriptError(stderr.String(), err),
		}
	}

	return wbclientgo.DomainOpsResp{Success: true}
}

func DomainJoin(ctx context.Context, req wbclientgo.DomainJoinReq) wbclientgo.DomainOpsResp {
	log.WithCtx(ctx).Printf("wbclient(domainjoin) - request: dcfqdn[%s] netbios[%s] user[%s]",
		req.DCFQDN, req.NetbiosName, req.ADUsername)

	if err := exec.CommandContext(ctx, "wbinfo", "-t").Run(); err == nil {
		log.WithCtx(ctx).Printf("wbclient(domainjoin) - already joined; skipping script")
		return wbclientgo.DomainOpsResp{
			Success:      true,
			ErrorMessage: "already joined",
		}
	}

	timeoutDays := req.MachinePasswordRefreshInterval
	if timeoutDays <= 0 {
		timeoutDays = defaultMachinePasswordTimeoutDays
	}
	timeoutSeconds := timeoutDays * secondsPerDay

	env := []string{
		"DC_FQDN=" + req.DCFQDN,
		"NETBIOS_NAME=" + req.NetbiosName,
		"AD_USERNAME=" + req.ADUsername,
		"AD_PASSWORD=" + req.ADPassword,
		fmt.Sprintf("MACHINE_PASSWORD_TIMEOUT=%d", timeoutSeconds),
	}

	return runScript(ctx, defaultDomainJoinScript, env, "domainjoin")
}

func DomainLeave(ctx context.Context, req wbclientgo.DomainLeaveReq) wbclientgo.DomainOpsResp {
	log.WithCtx(ctx).Printf("wbclient(domainleave) - request: domain[%s] user[%s]",
		req.Domain, req.ADUsername)

	env := []string{
		"AD_USERNAME=" + req.ADUsername,
		"AD_PASSWORD=" + req.ADPassword,
	}

	return runScript(ctx, defaultDomainLeaveScript, env, "domainleave")
}

func DomainJoinStatus(ctx context.Context) wbclientgo.DomainOpsResp {
	return runScript(ctx, defaultDomainJoinStatusScript, nil, "domainjoinstatus")
}

func SetLogLevel(ctx context.Context, req wbclientgo.SetLogLevelReq) wbclientgo.DomainOpsResp {
	log.WithCtx(ctx).Printf("wbclient(setloglevel) - request: logLevel[%d]", req.LogLevel)

	env := []string{
		fmt.Sprintf("SAMBA_LOG_LEVEL=%d", req.LogLevel),
	}

	return runScript(ctx, defaultSetLogLevelScript, env, "setloglevel")
}

func runMigrateConfigScript(ctx context.Context) wbclientgo.DomainOpsResp {
	return runScript(ctx, defaultMigrateConfigScript, nil, "migrateconfig")
}
