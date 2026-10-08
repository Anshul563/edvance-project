package service

import (
	"bytes"
	"fmt"
	"text/template"
)

// RenderTemplate renders Go text templates like
// "Your payment of {{.amount}} was successful." Missing variables are
// errors, not silent blanks: a template referencing data the event did
// not supply must fail loudly at render time rather than send a broken
// message.
func RenderTemplate(tmpl string, data map[string]string) (string, error) {
	parsed, err := template.New("notification").Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var rendered bytes.Buffer

	if err := parsed.Execute(&rendered, data); err != nil {
		return "", fmt.Errorf("render template: %w", err)
	}

	return rendered.String(), nil
}
