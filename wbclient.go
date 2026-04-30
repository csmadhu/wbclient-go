package wbclientgo

// UserValidateReq represents the request for test authentication
type UserValidateReq struct {
	Username        string `json:"username"`
	Password        string `json:"password"`
	Domain          string `json:"domain"`
	IsPlainTextAuth bool   `json:"isPlainTextAuth"`
}

// UserAuthReq represents the request for MSCHAPv2 authentication
type UserAuthReq struct {
	Username  string  `json:"username"`
	Domain    string  `json:"domain"`
	Challenge [8]byte `json:"challenge"`
	Response  []byte  `json:"response"`
}

// UserAuthResp represents the result of MSCHAPv2 authentication
type UserAuthResp struct {
	Success      bool     `json:"success"`
	ErrorCode    int      `json:"errorCode"`
	ErrorMessage string   `json:"errorMessage"`
	NTHashHash   [16]byte `json:"ntHashHash"`
}
