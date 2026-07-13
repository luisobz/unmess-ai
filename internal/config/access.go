package config

import (
	"fmt"
	"strconv"
	"strings"
)

// Get devuelve el valor de una clave con notación de punto (p. ej.
// "debounce_seconds", "retention.max_versions", "ui.port"). Las listas se
// devuelven separadas por comas.
func (c *Config) Get(key string) (string, error) {
	switch strings.ToLower(key) {
	case "prefix":
		return c.Prefix, nil
	case "debounce_seconds":
		return strconv.Itoa(c.DebounceSeconds), nil
	case "included_paths":
		return strings.Join(c.IncludedPaths, ","), nil
	case "excluded_paths":
		return strings.Join(c.ExcludedPaths, ","), nil
	case "exclude_names":
		return strings.Join(c.ExcludeNames, ","), nil
	case "gitignore_aware":
		return strconv.FormatBool(c.GitignoreAware), nil
	case "max_file_size_mb":
		return strconv.Itoa(c.MaxFileSizeMB), nil
	case "retention.max_versions":
		return strconv.Itoa(c.Retention.MaxVersions), nil
	case "retention.max_age_days":
		return strconv.Itoa(c.Retention.MaxAgeDays), nil
	case "retention.deleted_age_days":
		return strconv.Itoa(c.Retention.DeletedAgeDays), nil
	case "retention.min_keep":
		return strconv.Itoa(c.Retention.MinKeep), nil
	case "ui.port":
		return strconv.Itoa(c.UI.Port), nil
	default:
		return "", fmt.Errorf("clave desconocida: %q", key)
	}
}

// Set asigna el valor de una clave (misma notación que Get). Las listas aceptan
// valores separados por comas. No persiste: llama a Save después.
func (c *Config) Set(key, value string) error {
	switch strings.ToLower(key) {
	case "prefix":
		c.Prefix = value
	case "debounce_seconds":
		return setInt(&c.DebounceSeconds, value)
	case "included_paths":
		c.IncludedPaths = splitList(value)
	case "excluded_paths":
		c.ExcludedPaths = splitList(value)
	case "exclude_names":
		c.ExcludeNames = splitList(value)
	case "gitignore_aware":
		return setBool(&c.GitignoreAware, value)
	case "max_file_size_mb":
		return setInt(&c.MaxFileSizeMB, value)
	case "retention.max_versions":
		return setInt(&c.Retention.MaxVersions, value)
	case "retention.max_age_days":
		return setInt(&c.Retention.MaxAgeDays, value)
	case "retention.deleted_age_days":
		return setInt(&c.Retention.DeletedAgeDays, value)
	case "retention.min_keep":
		return setInt(&c.Retention.MinKeep, value)
	case "ui.port":
		return setInt(&c.UI.Port, value)
	default:
		return fmt.Errorf("clave desconocida: %q", key)
	}
	return nil
}

func setInt(dst *int, value string) error {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("valor entero inválido %q: %w", value, err)
	}
	*dst = n
	return nil
}

func setBool(dst *bool, value string) error {
	b, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("valor booleano inválido %q: %w", value, err)
	}
	*dst = b
	return nil
}

func splitList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
