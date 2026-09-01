package internal

/*
#cgo darwin CFLAGS: -I/opt/homebrew/opt/samba/include
#cgo darwin LDFLAGS: -L/opt/homebrew/opt/samba/lib -lwbclient
#cgo linux CFLAGS: -I/usr/include/samba-4.0
#cgo linux LDFLAGS: -lwbclient
#include <stdlib.h>
#include <stdint.h>
#include <stdbool.h>
#include <string.h>
#include <stdio.h>
#include <wbclient.h>

// NT Status codes
#define NT_STATUS_PASSWORD_EXPIRED 0xC0000071
#define NT_STATUS_PASSWORD_MUST_CHANGE 0xC0000224

// Constants for parameter control flags
#define WBC_MSV1_0_ALLOW_MSVCHAPV2 0x00010000
#define WBC_MSV1_0_ALLOW_WORKSTATION_TRUST_ACCOUNT 0x00000800
// WBC_MSV1_0_ALLOW_SERVER_TRUST_ACCOUNT is already defined in wbclient.h

// NT_LENGTH constant from FreeRADIUS
#define NT_LENGTH 24
#define NT_DIGEST_LENGTH 16

// Wrapper function for MSCHAPv2 authentication via winbind
// Follows FreeRADIUS rlm_mschap implementation
int go_wbc_auth_mschapv2(
    const char *username,
    const char *domain,
    const uint8_t *challenge,
    const uint8_t *response,
    int response_len,
    uint8_t *nthashhash,
    char *error_msg,
    int error_msg_len,
    uint32_t *out_nt_status
) {
    struct wbcAuthUserParams authparams;
    struct wbcAuthUserInfo *info = NULL;
    struct wbcAuthErrorInfo *error = NULL;
    wbcErr err;

    // Allocate buffer for the response
    // MSCHAPv2: 24 bytes (NT-Response)
    // Can also support NTLMv2: 48+ bytes (NTProofStr + blob)
    uint8_t *resp = (uint8_t *)malloc(response_len);

    // Initialize auth parameters structure
    memset(&authparams, 0, sizeof(authparams));

    // Set account and domain name
    authparams.account_name = (char *)username;
    authparams.domain_name = (char *)domain;
    authparams.workstation_name = NULL;
    authparams.flags = 0;

    // Configure authentication level and method
    authparams.level = WBC_AUTH_USER_LEVEL_RESPONSE;

    // For MSCHAPv2 via wbclient (same as FreeRADIUS):
    // - Challenge is the 8-byte ChallengeHash (SHA1 of peer + auth + username)
    // - Response is the 24-byte NT-Response (DESL of challenge hash with NT hash)
    // Note: Can also support NTLMv2 with 48+ byte responses
    authparams.password.response.nt_length = response_len;

    // Copy the response
    memcpy(resp, response, response_len);
    authparams.password.response.nt_data = resp;

    // Copy the 8-byte challenge (ChallengeHash for MSCHAPv2)
    memcpy(authparams.password.response.challenge, challenge, 8);

    // Set parameter control flags for MSCHAPv2
    authparams.parameter_control |= WBC_MSV1_0_ALLOW_MSVCHAPV2 |
                                    WBC_MSV1_0_ALLOW_WORKSTATION_TRUST_ACCOUNT |
                                    WBC_MSV1_0_ALLOW_SERVER_TRUST_ACCOUNT;

    // Authenticate user via winbind
    err = wbcAuthenticateUserEx(&authparams, &info, &error);

    // Process the authentication result
    *out_nt_status = 0;
    int rcode = -1;
    switch (err) {
    case WBC_ERR_SUCCESS:
        rcode = 0;
        memcpy(nthashhash, info->user_session_key, NT_DIGEST_LENGTH);
        break;

    case WBC_ERR_WINBIND_NOT_AVAILABLE:
        rcode = -2;
        snprintf(error_msg, error_msg_len, "Winbind is not available");
        break;

    case WBC_ERR_DOMAIN_NOT_FOUND:
        rcode = -1;
        snprintf(error_msg, error_msg_len, "Domain not found");
        break;

    case WBC_ERR_AUTH_ERROR:
        rcode = -1;
        if (error) {
            *out_nt_status = error->nt_status;
            if (error->nt_status == NT_STATUS_PASSWORD_EXPIRED ||
                error->nt_status == NT_STATUS_PASSWORD_MUST_CHANGE) {
                rcode = -648;
            }
            if (error->display_string) {
                snprintf(error_msg, error_msg_len, "%s [0x%X]",
                        error->display_string, error->nt_status);
            } else {
                snprintf(error_msg, error_msg_len, "Authentication failed [0x%X]",
                        error->nt_status);
            }
        } else {
            snprintf(error_msg, error_msg_len, "Authentication failed");
        }
        break;

    default:
        rcode = -2;
        if (error && error->display_string) {
            snprintf(error_msg, error_msg_len, "libwbclient error: %s",
                    error->display_string);
        } else {
            snprintf(error_msg, error_msg_len, "libwbclient error: %d", err);
        }
        break;
    }

    // Cleanup
    if (resp) {
        free(resp);
    }
    if (error) {
        wbcFreeMemory(error);
    }
    if (info) {
        wbcFreeMemory(info);
    }

    return rcode;
}

// Wrapper function for plaintext authentication via winbind
// Uses wbcAuthenticateUserEx to capture NT status error details
int go_wbc_auth_plaintext(
    const char *username,
    const char *domain,
    const char *password,
    char *error_msg,
    int error_msg_len,
    uint32_t *out_nt_status
) {
    struct wbcAuthUserParams authparams;
    struct wbcAuthUserInfo *info = NULL;
    struct wbcAuthErrorInfo *error = NULL;
    wbcErr err;

    memset(&authparams, 0, sizeof(authparams));

    authparams.account_name = (char *)username;
    authparams.domain_name = (char *)domain;
    authparams.workstation_name = NULL;
    authparams.flags = 0;
    authparams.level = WBC_AUTH_USER_LEVEL_PLAIN;
    authparams.password.plaintext = (char *)password;

    err = wbcAuthenticateUserEx(&authparams, &info, &error);

    *out_nt_status = 0;
    int rcode = -1;
    switch (err) {
    case WBC_ERR_SUCCESS:
        rcode = 0;
        break;

    case WBC_ERR_WINBIND_NOT_AVAILABLE:
        rcode = -2;
        snprintf(error_msg, error_msg_len, "Winbind is not available");
        break;

    case WBC_ERR_DOMAIN_NOT_FOUND:
        rcode = -1;
        snprintf(error_msg, error_msg_len, "Domain not found");
        break;

    case WBC_ERR_AUTH_ERROR:
        rcode = -1;
        if (error) {
            *out_nt_status = error->nt_status;
            if (error->nt_status == NT_STATUS_PASSWORD_EXPIRED ||
                error->nt_status == NT_STATUS_PASSWORD_MUST_CHANGE) {
                rcode = -648;
            }
            if (error->display_string) {
                snprintf(error_msg, error_msg_len, "%s [0x%X]",
                        error->display_string, error->nt_status);
            } else {
                snprintf(error_msg, error_msg_len, "Authentication failed [0x%X]",
                        error->nt_status);
            }
        } else {
            snprintf(error_msg, error_msg_len, "Authentication failed");
        }
        break;

    default:
        rcode = -2;
        if (error && error->display_string) {
            snprintf(error_msg, error_msg_len, "libwbclient error: %s",
                    error->display_string);
        } else {
            snprintf(error_msg, error_msg_len, "libwbclient error: %d", err);
        }
        break;
    }

    if (error) {
        wbcFreeMemory(error);
    }
    if (info) {
        wbcFreeMemory(info);
    }

    return rcode;
}
*/
import "C"

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"
	"unsafe"

	wbclientgo "github.com/csmadhu/wbclient-go"
	"github.com/csmadhu/wbclient-go/log"
)

var wbThrottler = NewWinbindThrottler(envInt("WBCLIENT_MAX_CONCURRENT_AUTH", 400))

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func AuthenticateMSCHAPv2(ctx context.Context, req wbclientgo.UserAuthReq) (result wbclientgo.UserAuthResp) {
	t := time.Now()
	log.WithCtx(ctx).Printf("wbclient - authenticate mschapv2: username:%s domain:%s challenge:%x response:%x", req.Username, req.Netbios,
		req.Challenge, req.Response)

	if req.Username == "" || req.Netbios == "" {
		result.ErrorMessage = "Username and domain required"
		result.ErrorCode = -1
		return result
	}

	if err := wbThrottler.Acquire(ctx); err != nil {
		result.ErrorCode = -2
		result.ErrorMessage = fmt.Sprintf("Request cancelled while waiting for winbind: %v", err)
		return result
	}
	defer wbThrottler.Release()

	cUsername := C.CString(req.Username)
	cDomain := C.CString(req.Netbios)
	defer C.free(unsafe.Pointer(cUsername))
	defer C.free(unsafe.Pointer(cDomain))

	cChallenge := (*C.uint8_t)(unsafe.Pointer(&req.Challenge[0]))
	cResponse := (*C.uint8_t)(unsafe.Pointer(&req.Response[0]))
	cResponseLen := C.int(len(req.Response))

	errorBuf := make([]byte, 256)
	cErrorMsg := (*C.char)(unsafe.Pointer(&errorBuf[0]))

	var ntHashHash [16]byte
	cNTHashHash := (*C.uint8_t)(unsafe.Pointer(&ntHashHash[0]))

	var outNTStatus C.uint32_t

	rcode := int(C.go_wbc_auth_mschapv2(
		cUsername, cDomain, cChallenge, cResponse, cResponseLen,
		cNTHashHash, cErrorMsg, 256, &outNTStatus,
	))

	result.ErrorCode = rcode
	result.NTHashHash = ntHashHash
	result.Success = (rcode == 0)

	errMsg := C.GoString(cErrorMsg)
	ntStatus := uint32(outNTStatus)
	if ntStatus != 0 {
		result.ErrorMessage = fmt.Sprintf("%s: %s", ntStatusBaseError(ntStatus), errMsg)
	} else {
		result.ErrorMessage = errMsg
	}

	log.WithCtx(ctx).Printf("wbclient - authenticate mschapv2 completed: result:%+v duration:%v", result, time.Since(t))
	return result
}

func AuthenticateWithChallenge(ctx context.Context, req wbclientgo.UserValidateReq) wbclientgo.UserAuthResp {
	if req.Username == "" || req.Password == "" {
		result := wbclientgo.UserAuthResp{
			ErrorMessage: "Username and password required",
			ErrorCode:    -1,
			Success:      false,
		}
		log.WithCtx(ctx).Errorf("wbclient - auth with challenge validation failed: username or password empty")
		return result
	}

	log.WithCtx(ctx).Printf("wbclient - authenticate with challenge: username[%s] domain[%s]", req.Username, req.Netbios)

	challenge, err := GenerateRandomChallenge()
	if err != nil {
		return wbclientgo.UserAuthResp{
			Success:      false,
			ErrorCode:    -1,
			ErrorMessage: fmt.Sprintf("Failed to generate challenge: %v", err),
		}
	}

	ntResponse := GenerateNTResponseSimple(challenge, req.Password)

	result := AuthenticateMSCHAPv2(ctx, wbclientgo.UserAuthReq{
		Username:  req.Username,
		Netbios:    req.Netbios,
		Challenge: challenge,
		Response:  ntResponse[:],
	})

	return result
}

func AuthenticateWithPlainText(ctx context.Context, req wbclientgo.UserValidateReq) wbclientgo.UserAuthResp {
	if req.Username == "" || req.Password == "" {
		result := wbclientgo.UserAuthResp{
			ErrorMessage: "Username and password required",
			ErrorCode:    -1,
			Success:      false,
		}
		log.WithCtx(ctx).Errorf("wbclient - plaintext auth validation failed: username or password empty")
		return result
	}

	log.WithCtx(ctx).Printf("wbclient - plaintext auth: username[%s] domain[%s]", req.Username, req.Netbios)

	if err := wbThrottler.Acquire(ctx); err != nil {
		return wbclientgo.UserAuthResp{
			Success:      false,
			ErrorCode:    -2,
			ErrorMessage: fmt.Sprintf("Request cancelled while waiting for winbind: %v", err),
		}
	}
	defer wbThrottler.Release()

	cUsername := C.CString(req.Username)
	cDomain := C.CString(req.Netbios)
	cPassword := C.CString(req.Password)
	defer C.free(unsafe.Pointer(cUsername))
	defer C.free(unsafe.Pointer(cDomain))
	defer C.free(unsafe.Pointer(cPassword))

	errorBuf := make([]byte, 256)
	cErrorMsg := (*C.char)(unsafe.Pointer(&errorBuf[0]))
	var outNTStatus C.uint32_t

	rcode := int(C.go_wbc_auth_plaintext(
		cUsername, cDomain, cPassword, cErrorMsg, 256, &outNTStatus,
	))

	result := wbclientgo.UserAuthResp{
		ErrorCode: rcode,
		Success:   rcode == 0,
	}

	errMsg := C.GoString(cErrorMsg)
	ntStatus := uint32(outNTStatus)
	if ntStatus != 0 {
		result.ErrorMessage = fmt.Sprintf("%s: %s", ntStatusBaseError(ntStatus), errMsg)
	} else {
		result.ErrorMessage = errMsg
	}

	if result.Success {
		log.WithCtx(ctx).Printf("wbclient - plaintext auth succeeded for username[%s]", req.Username)
	} else {
		log.WithCtx(ctx).Errorf("wbclient - plaintext auth failed: %s", result.ErrorMessage)
	}

	return result
}
