package spec

import (
	"encoding/json"
	"fmt"
)

type CommandSpec struct {
	SchemaVersion int       `json:"schemaVersion"`
	Kind          string    `json:"kind"`
	Metadata      Metadata  `json:"metadata"`
	Defaults      *Defaults `json:"defaults,omitempty"`
	Dependencies  []string  `json:"dependencies,omitempty"`
	Args          Args      `json:"args"`
	Steps         []Step    `json:"steps"`
	Policy        *Policy   `json:"policy,omitempty"`
}

type Metadata struct {
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Aliases     []string `json:"aliases,omitempty"`
}

type Defaults struct {
	Shell   string            `json:"shell,omitempty"`
	Timeout string            `json:"timeout,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

type Args struct {
	Positional []PositionalArg `json:"positional,omitempty"`
	Flags      []FlagArg       `json:"flags,omitempty"`
}

type PositionalArg struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    *bool  `json:"required,omitempty"`
	Default     string `json:"default,omitempty"`
}

func (p PositionalArg) IsRequired() bool {
	if p.Required == nil {
		return true
	}
	return *p.Required
}

type FlagArg struct {
	Name        string `json:"name"`
	Short       string `json:"short,omitempty"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"`
	Default     any    `json:"default,omitempty"`
	Required    *bool  `json:"required,omitempty"`
}

func (f FlagArg) IsRequired() bool {
	if f.Required == nil {
		return false
	}
	return *f.Required
}

func (f FlagArg) GetType() string {
	if f.Type == "" {
		return "string"
	}
	return f.Type
}

type Step struct {
	Name            string            `json:"name"`
	Run             string            `json:"run"`
	Env             map[string]string `json:"env,omitempty"`
	Timeout         string            `json:"timeout,omitempty"`
	ContinueOnError bool              `json:"continueOnError,omitempty"`
	Shell           string            `json:"shell,omitempty"`
}

type Policy struct {
	RequireConfirmation bool     `json:"requireConfirmation,omitempty"`
	AllowedExecutables  []string `json:"allowedExecutables,omitempty"`
}

func Parse(data []byte) (*CommandSpec, error) {
	// Normalize YAML to JSON once, then use JSON for both validation and unmarshal.
	jsonData, err := ToJSON(data)
	if err != nil {
		return nil, fmt.Errorf("format conversion failed: %w", err)
	}
	if err := Validate(jsonData); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}
	var s CommandSpec
	if err := json.Unmarshal(jsonData, &s); err != nil {
		return nil, fmt.Errorf("unmarshal failed: %w", err)
	}
	return &s, nil
}
