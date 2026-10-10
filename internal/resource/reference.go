// Package resource identifies infrastructure independently of any provider SDK.
package resource

// SourceID is stable configuration, not a hostname inferred during collection.
type SourceID struct {
	Environment string `json:"environment"`
	Provider    string `json:"provider"`
	Instance    string `json:"instance"`
}

// Reference is comparable and can therefore be used as a Go map key.
type Reference struct {
	Source SourceID `json:"source"`
	ID     string   `json:"id"`
}
