package internalapi

// ResourceDatasourceResponse is the reply to GET /internal/resource-datasource:
// the datasource id that governs access to a shareable resource. Peer services
// (the auth service's sharing ownership guard) resolve a resource to its
// datasource here and then verify the caller's datasource access.
type ResourceDatasourceResponse struct {
	DatasourceID string `json:"datasource_id"`
}
