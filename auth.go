package wbclientgo

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

// Helper functions to extract data from wbcAuthUserInfo and wbcAuthErrorInfo
static inline void copy_user_session_key(struct wbcAuthUserInfo *info, uint8_t *dest) {
    if (info != NULL) {
        memcpy(dest, info->user_session_key, NT_DIGEST_LENGTH);
    }
}

static inline uint32_t get_nt_status(struct wbcAuthErrorInfo *error) {
    return error ? error->nt_status : 0;
}

static inline const char* get_display_string(struct wbcAuthErrorInfo *error) {
    return (error && error->display_string) ? error->display_string : NULL;
}

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
    int error_msg_len
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
*/
import "C"

import (
	"context"
	"fmt"
	"time"
	"unsafe"

	"github.com/madhucs/wbclient-go/log"
)

const (
	// NT Status codes
	NT_STATUS_PASSWORD_EXPIRED     = 0xC0000071
	NT_STATUS_PASSWORD_MUST_CHANGE = 0xC0000224

	// NT hash length
	NT_DIGEST_LENGTH = 16
)

// processWbcAuthError is a reusable function to process wbcErr with detailed error info
// This function handles error processing for both MSCHAPv2 and plain text authentication
func processWbcAuthError(err C.wbcErr, detailedErr *C.struct_wbcAuthErrorInfo) (result UserAuthResp) {
	switch err {
	case C.WBC_ERR_SUCCESS:
		result.Success = true
		result.ErrorCode = 0
		result.ErrorMessage = ""

	case C.WBC_ERR_WINBIND_NOT_AVAILABLE:
		result.Success = false
		result.ErrorCode = -2
		result.ErrorMessage = "Winbind is not available"

	case C.WBC_ERR_DOMAIN_NOT_FOUND:
		result.Success = false
		result.ErrorCode = -1
		result.ErrorMessage = "Domain not found"

	case C.WBC_ERR_AUTH_ERROR:
		result.Success = false
		result.ErrorCode = -1
		if detailedErr != nil {
			ntStatus := uint32(C.get_nt_status(detailedErr))

			// Check for password expiry conditions
			if ntStatus == NT_STATUS_PASSWORD_EXPIRED || ntStatus == NT_STATUS_PASSWORD_MUST_CHANGE {
				result.ErrorCode = -648
			}

			displayString := C.get_display_string(detailedErr)
			if displayString != nil {
				result.ErrorMessage = fmt.Sprintf("%s [0x%X]", C.GoString(displayString), ntStatus)
			} else {
				result.ErrorMessage = fmt.Sprintf("Authentication failed [0x%X]", ntStatus)
			}
		} else {
			result.ErrorMessage = "Authentication failed"
		}

	default:
		result.Success = false
		result.ErrorCode = -2
		if detailedErr != nil {
			displayString := C.get_display_string(detailedErr)
			if displayString != nil {
				result.ErrorMessage = fmt.Sprintf("libwbclient error: %s", C.GoString(displayString))
			} else {
				result.ErrorMessage = fmt.Sprintf("libwbclient error: %d", int(err))
			}
		} else {
			result.ErrorMessage = fmt.Sprintf("libwbclient error: %d", int(err))
		}
	}

	return result
}

// AuthenticateMSCHAPv2 performs MSCHAPv2 authentication
func AuthenticateMSCHAPv2(ctx context.Context, req UserAuthReq) (result UserAuthResp) {
	t := time.Now()
	log.WithCtx(ctx).Printf("wbclient - authenticate mschapv2: username:%s domain:%s challenge:%x response:%x", req.Username, req.Domain, req.Challenge, req.Response)

	if req.Username == "" || req.Domain == "" {
		result.ErrorMessage = "Username and domain required"
		result.ErrorCode = -1
		return result
	}

	cUsername := C.CString(req.Username)
	cDomain := C.CString(req.Domain)
	defer C.free(unsafe.Pointer(cUsername))
	defer C.free(unsafe.Pointer(cDomain))

	cChallenge := (*C.uint8_t)(unsafe.Pointer(&req.Challenge[0]))
	cResponse := (*C.uint8_t)(unsafe.Pointer(&req.Response[0]))
	cResponseLen := C.int(len(req.Response))

	errorBuf := make([]byte, 256)
	cErrorMsg := (*C.char)(unsafe.Pointer(&errorBuf[0]))

	var ntHashHash [16]byte
	cNTHashHash := (*C.uint8_t)(unsafe.Pointer(&ntHashHash[0]))

	rcode := int(C.go_wbc_auth_mschapv2(
		cUsername, cDomain, cChallenge, cResponse, cResponseLen,
		cNTHashHash, cErrorMsg, 256,
	))

	result.ErrorCode = rcode
	result.NTHashHash = ntHashHash
	result.ErrorMessage = C.GoString(cErrorMsg)
	result.Success = (rcode == 0)

	log.WithCtx(ctx).Printf("wbclient - authenticate mschapv2 completed: result:%+v duration:%v", result, time.Since(t))
	return result
}

// AuthenticateWithChallenge performs MSCHAPv2 authentication using username, domain, and password.
// This function replicates the flow from cmd/authtest/main.go:
// 1. Generate a random 8-byte challenge
// 2. Generate the NT-Response from the password
// 3. Call AuthenticateMSCHAPv2 to verify credentials
//
// Parameters:
//   - ctx: context for logging with metadata
//   - username: username to authenticate
//   - domain: domain name (NetBIOS name, not DNS name)
//   - password: plaintext password
//
// Returns:
//   - AuthResult: authentication result with success status and error details
func AuthenticateWithChallenge(ctx context.Context, req UserValidateReq) UserAuthResp {
	// Validate input
	if req.Username == "" || req.Password == "" {
		result := UserAuthResp{
			ErrorMessage: "Username and password required",
			ErrorCode:    -1,
			Success:      false,
		}
		log.WithCtx(ctx).Errorf("wbclient - auth with challenge validation failed: username or password empty")
		return result
	}

	log.WithCtx(ctx).Printf("wbclient - authenticate with challenge: username[%s] domain[%s]", req.Username, req.Domain)

	// Step 1: Generate a random 8-byte challenge
	challenge, err := GenerateRandomChallenge()
	if err != nil {
		return UserAuthResp{
			Success:      false,
			ErrorCode:    -1,
			ErrorMessage: fmt.Sprintf("Failed to generate challenge: %v", err),
		}
	}

	// Step 2: Generate the NT-Response using the password
	// This computes: ChallengeResponse(Challenge, MD4(UTF-16LE(Password)))
	ntResponse := GenerateNTResponseSimple(challenge, req.Password)

	// Step 3: Authenticate using Winbind
	// This calls the Winbind library (libwbclient) to verify the credentials
	// against Active Directory via the domain controller
	result := AuthenticateMSCHAPv2(ctx, UserAuthReq{
		Username:  req.Username,
		Domain:    req.Domain,
		Challenge: challenge,
		Response:  ntResponse[:],
	})

	return result
}

// AuthenticateWithPlainText performs plain text password authentication using wbclient library.
//
// Parameters:
//   - ctx: context for logging with metadata
//   - username: username to authenticate
//   - domain: domain name (kept for API consistency, not used in wbcAuthenticateUser)
//   - password: plaintext password
//
// Returns:
//   - AuthResp: authentication result
func AuthenticateWithPlainText(ctx context.Context, req UserValidateReq) UserAuthResp {
	// Validate input
	if req.Username == "" || req.Password == "" {
		result := UserAuthResp{
			ErrorMessage: "Username and password required",
			ErrorCode:    -1,
			Success:      false,
		}
		log.WithCtx(ctx).Errorf("wbclient - plaintext auth validation failed: username or password empty")
		return result
	}

	log.WithCtx(ctx).Printf("wbclient - plaintext auth: username[%s] domain[%s]", req.Username, req.Domain)

	// Convert Go strings to C strings
	cUsername := C.CString(req.Username)
	cPassword := C.CString(req.Password)
	defer C.free(unsafe.Pointer(cUsername))
	defer C.free(unsafe.Pointer(cPassword))

	// Call wbcAuthenticateUser directly
	err := C.wbcAuthenticateUser(cUsername, cPassword)

	// Process the authentication result using the reusable function
	// No error struct available for plain text auth
	result := processWbcAuthError(err, nil)

	// Log the result
	if result.Success {
		log.WithCtx(ctx).Printf("wbclient - plaintext auth succeeded for username[%s]", req.Username)
	} else {
		log.WithCtx(ctx).Errorf("wbclient - plaintext auth failed: %s", result.ErrorMessage)
	}

	return result
}
