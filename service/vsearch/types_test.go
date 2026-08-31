package vsearch

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSearchProviderReservedParam(t *testing.T) {
	tests := []struct {
		name     string
		reserved bool
	}{
		{name: " token ", reserved: true},
		{name: "Authorization", reserved: true},
		{name: "provider_params", reserved: true},
		{name: "raw", reserved: true},
		{name: "query", reserved: false},
		{name: "limit", reserved: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.reserved, searchProviderReservedParam(test.name))
		})
	}
}
