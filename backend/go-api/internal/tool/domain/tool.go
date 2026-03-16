package domain

type Parameter struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Default     any    `json:"default,omitempty"`
}

type Tool struct {
	Name              string      `json:"name"`
	DisplayName       string      `json:"display_name"`
	Description       string      `json:"description"`
	Category          string      `json:"category"`
	AllowedRoles      []string    `json:"allowed_roles"`
	Parameters        []Parameter `json:"parameters"`
	Available         bool        `json:"available"`
	UnavailableReason string      `json:"unavailable_reason,omitempty"`
}
