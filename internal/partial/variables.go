package partial

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tormodhaugland/co/internal/git"
	"github.com/tormodhaugland/co/internal/template"
)

// GetPartialBuiltins returns built-in variables for partial templates.
func GetPartialBuiltins(targetPath string) (map[string]string, error) {
	now := time.Now()
	vars := map[string]string{
		"DATE":      now.Format("2006-01-02"),
		"DATETIME":  now.Format(time.RFC3339),
		"YEAR":      now.Format("2006"),
		"TIMESTAMP": fmt.Sprintf("%d", now.Unix()),
	}

	if home, err := os.UserHomeDir(); err == nil {
		vars["HOME"] = home
	}

	if name := getGitConfig("user.name"); name != "" {
		vars["GIT_USER_NAME"] = name
	}
	if email := getGitConfig("user.email"); email != "" {
		vars["GIT_USER_EMAIL"] = email
	}

	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return nil, fmt.Errorf("resolving target path: %w", err)
	}

	vars["DIRNAME"] = filepath.Base(absTarget)
	vars["DIRPATH"] = absTarget
	vars["PARENT_DIRNAME"] = filepath.Base(filepath.Dir(absTarget))

	if git.IsRepo(absTarget) {
		vars["IS_GIT_REPO"] = "true"
		info, err := git.GetInfo(absTarget)
		if err != nil {
			return vars, err
		}
		if info.Branch != "" {
			vars["GIT_BRANCH"] = info.Branch
		}
		if info.Remote != "" {
			vars["GIT_REMOTE_URL"] = info.Remote
		}
	} else {
		vars["IS_GIT_REPO"] = "false"
	}

	return vars, nil
}

// ResolvePartialVariables merges builtins, user-provided values, and defaults.
func ResolvePartialVariables(p *Partial, provided, builtins map[string]string) (map[string]string, error) {
	if provided == nil {
		provided = map[string]string{}
	}

	resolved := make(map[string]string)
	declared := make(map[string]struct{}, len(p.Variables))
	for k, v := range builtins {
		resolved[k] = v
	}

	for _, v := range p.Variables {
		declared[v.Name] = struct{}{}
		if value, ok := provided[v.Name]; ok {
			if err := template.ValidateVarValue(partialVarToTemplateVar(v), value); err != nil {
				return nil, mapTemplateVarError(err)
			}
			resolved[v.Name] = value
			continue
		}

		if v.Default != nil {
			defaultStr := fmt.Sprintf("%v", v.Default)
			processed, err := template.ProcessTemplateContent(defaultStr, resolved)
			if err != nil {
				return nil, fmt.Errorf("processing default for %s: %w", v.Name, err)
			}
			if err := template.ValidateVarValue(partialVarToTemplateVar(v), processed); err != nil {
				return nil, mapTemplateVarError(err)
			}
			resolved[v.Name] = processed
			continue
		}

		if v.Required {
			return nil, &MissingRequiredVarError{
				VarName:     v.Name,
				Description: v.Description,
			}
		}
	}

	for k, v := range provided {
		if _, ok := declared[k]; ok {
			continue
		}
		if _, ok := resolved[k]; ok {
			continue
		}
		resolved[k] = v
	}

	return resolved, nil
}

func partialVarToTemplateVar(v PartialVar) template.TemplateVar {
	return template.TemplateVar{
		Name:        v.Name,
		Description: v.Description,
		Type:        v.Type,
		Required:    v.Required,
		Default:     v.Default,
		Validation:  v.Validation,
		Choices:     v.Choices,
	}
}

func mapTemplateVarError(err error) error {
	var tmplErr *template.InvalidVarValueError
	if errors.As(err, &tmplErr) {
		return &InvalidVarValueError{
			VarName:    tmplErr.VarName,
			Value:      tmplErr.Value,
			Validation: tmplErr.Validation,
			Reason:     tmplErr.Reason,
		}
	}
	return err
}

// Note: getGitConfig is defined in apply.go to avoid duplication
