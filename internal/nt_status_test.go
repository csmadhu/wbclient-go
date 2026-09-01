package internal

import (
	"fmt"
	"testing"
)

func TestNtStatusBaseError(t *testing.T) {
	tests := []struct {
		ntStatus uint32
		want     string
	}{
		{0xC0000022, "NT_STATUS_ACCESS_DENIED"},
		{0xC000005E, "NT_STATUS_NO_LOGON_SERVERS"},
		{0xC0000064, "NT_STATUS_NO_SUCH_USER"},
		{0xC000006A, "NT_STATUS_WRONG_PASSWORD"},
		{0xC000006D, "NT_STATUS_LOGON_FAILURE"},
		{0xC000006E, "NT_STATUS_ACCOUNT_RESTRICTION"},
		{0xC0000071, "NT_STATUS_PASSWORD_EXPIRED"},
		{0xC0000072, "NT_STATUS_ACCOUNT_DISABLED"},
		{0xC000015B, "NT_STATUS_LOGON_TYPE_NOT_GRANTED"},
		{0xC0000133, "NT_STATUS_TIME_DIFFERENCE_AT_DC"},
		{0xC0000193, "NT_STATUS_ACCOUNT_EXPIRED"},
		{0xC0000199, "NT_STATUS_NOLOGON_WORKSTATION_TRUST_ACCOUNT"},
		{0xC000019B, "NT_STATUS_NOLOGON_INTERDOMAIN_TRUST_ACCOUNT"},
		{0xC0000224, "NT_STATUS_PASSWORD_MUST_CHANGE"},
		{0xC0000234, "NT_STATUS_ACCOUNT_LOCKED_OUT"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := ntStatusBaseError(tt.ntStatus)
			if got != tt.want {
				t.Errorf("ntStatusBaseError(0x%X) = %q, want %q", tt.ntStatus, got, tt.want)
			}
		})
	}
}

func TestNtStatusBaseErrorUnknown(t *testing.T) {
	unknownCodes := []uint32{0xC0000001, 0xDEADBEEF, 0x00000000}

	for _, code := range unknownCodes {
		t.Run(fmt.Sprintf("0x%X", code), func(t *testing.T) {
			got := ntStatusBaseError(code)
			want := fmt.Sprintf("NT_STATUS_UNKNOWN(0x%X)", code)
			if got != want {
				t.Errorf("ntStatusBaseError(0x%X) = %q, want %q", code, got, want)
			}
		})
	}
}
