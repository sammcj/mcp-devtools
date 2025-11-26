package auth

// ClientInfo holds OAuth client registration information.
type ClientInfo struct {
	ClientID                string   `json:"client_id"`
	ClientSecret            string   `json:"client_secret,omitempty"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at,omitempty"`
	ClientSecretExpiresAt   int64    `json:"client_secret_expires_at,omitempty"`
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
	GrantTypes              []string `json:"grant_types,omitempty"`
	ResponseTypes           []string `json:"response_types,omitempty"`
	ClientName              string   `json:"client_name,omitempty"`
	ClientURI               string   `json:"client_uri,omitempty"`
	Scope                   string   `json:"scope,omitempty"`
}

// ClientMetadata holds OAuth client metadata for registration.
type ClientMetadata struct {
	RedirectURIs            []string `json:"redirect_uris"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty"`
	GrantTypes              []string `json:"grant_types,omitempty"`
	ResponseTypes           []string `json:"response_types,omitempty"`
	ClientName              string   `json:"client_name,omitempty"`
	ClientURI               string   `json:"client_uri,omitempty"`
	Scope                   string   `json:"scope,omitempty"`
	SoftwareID              string   `json:"software_id,omitempty"`
	SoftwareVersion         string   `json:"software_version,omitempty"`
}

// SaveClientInfo persists client info to the cache directory.
func SaveClientInfo(cacheDir, serverHash string, info *ClientInfo) error {
	return WriteJSON(cacheDir, serverHash, "client_info.json", info)
}

// LoadClientInfo loads client info from the cache directory.
func LoadClientInfo(cacheDir, serverHash string) (*ClientInfo, error) {
	var info ClientInfo
	if err := ReadJSON(cacheDir, serverHash, "client_info.json", &info); err != nil {
		return nil, err
	}
	return &info, nil
}

// DeleteClientInfo removes stored client info.
func DeleteClientInfo(cacheDir, serverHash string) error {
	return DeleteFile(cacheDir, serverHash, "client_info.json")
}
