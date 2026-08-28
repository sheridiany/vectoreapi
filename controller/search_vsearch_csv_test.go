package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeSearchCSVTextNeutralizesSpreadsheetFormulas(t *testing.T) {
	tests := map[string]string{
		"=SUM(A1:A2)":         "'=SUM(A1:A2)",
		" +cmd|' /C calc'!A0": "' +cmd|' /C calc'!A0",
		"\t-2+3":              "'\t-2+3",
		"\n@danger":           "'\n@danger",
		"Brave Search":        "Brave Search",
		"":                    "",
	}
	for input, expected := range tests {
		assert.Equal(t, expected, sanitizeSearchCSVText(input))
	}
}
