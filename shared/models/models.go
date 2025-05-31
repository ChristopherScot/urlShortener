package models

type Link struct {
	Alias       string `json:"alias"`
	TargetURL   string `json:"target_url"`
	Description string `json:"description,omitempty"`
	Creator     string `json:"creator,omitempty"`
}
