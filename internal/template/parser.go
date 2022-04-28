package template

import (
	"bytes"
	"text/template"
)

func ParseTemplate(text string, data interface{}) (string, error) {
	tmpl, err := template.New("test").Parse(text)
	if err != nil {
		return "", err
	}

	var tpl bytes.Buffer
	if err = tmpl.Execute(&tpl, data); err != nil {
		return "", err
	}

	return tpl.String(), nil
}
