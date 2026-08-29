package vsearch

import (
	"strings"

	"github.com/QuantumNous/new-api/model"
)

const blockedSearchToolProvider = "JustOneAPI"

func searchToolAllowed(toolName string) bool {
	provider, _, _ := strings.Cut(strings.TrimSpace(toolName), "/")
	return !strings.EqualFold(provider, blockedSearchToolProvider)
}

func listAllowedSearchBindingIDs(capabilityID int) ([]int, error) {
	bindings, err := model.ListSearchCapabilityBindings(capabilityID, true)
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(bindings))
	for _, binding := range bindings {
		if searchToolAllowed(binding.ToolName) {
			ids = append(ids, binding.Id)
		}
	}
	return ids, nil
}
