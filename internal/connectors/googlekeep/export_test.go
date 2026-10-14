package googlekeep

import "github.com/brendanjerwin/simple_wiki/internal/connectors/googlekeep/gateway"

// FlattenRawIDMapForTest exposes flattenRawIDMap so the external test
// package can exercise all three switch arms (map[string]string,
// map[string]any, and the default nil path) without having to drive a
// full preserveItemIDMap integration scenario.
func FlattenRawIDMapForTest(raw any) map[string]string {
	return flattenRawIDMap(raw)
}

// IsLiveListItemForParentForTest exposes isLiveListItemForParent for
// targeted unit tests covering the Deleted-timestamp and empty-ServerID
// branches that are not reached by the higher-level SeedBindingState
// integration tests.
func IsLiveListItemForParentForTest(n gateway.Node, parentID string) bool {
	return isLiveListItemForParent(n, parentID)
}
