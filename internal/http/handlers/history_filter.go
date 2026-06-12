package handlers

import (
	"slices"

	pkgmetadata "github.com/biqly/biqly/pkg/metadata"
	pkgquery "github.com/biqly/biqly/pkg/query"
)

const (
	PermissionAIViewDetails = "ai:queue:view_details"
	PermissionAIViewStatus  = "ai:queue:view_status"
)

// FilterAIHistoryForUser returns the subset of rows visible to the caller and
// masks sensitive fields (prompt, AI response, logical query) on rows the user
// does not own when they lack the ai:queue:view_details permission.
//
// userID is the authenticated user's ID (may be empty in legacy API-key mode).
// permissions is the caller's effective permission list.
//
// Behavior:
//   - If permissions contains ai:queue:view_details → full visibility, no
//     masking, no filtering (admin/super_admin path).
//   - Else if userID is empty (no JWT context) → fall back to legacy behavior:
//     return rows unchanged. Migration period only.
//   - Else → drop rows belonging to other users; on remaining rows leave fields
//     intact since the user is the owner.
func FilterAIHistoryForUser(rows []pkgmetadata.AIQueryHistoryEntry, userID string, permissions []string) []pkgmetadata.AIQueryHistoryEntry {
	if slices.Contains(permissions, PermissionAIViewDetails) {
		return rows
	}
	if userID == "" {
		return rows
	}

	result := make([]pkgmetadata.AIQueryHistoryEntry, 0, len(rows))
	for _, row := range rows {
		if row.UserID != nil && *row.UserID == userID {
			result = append(result, row)
		}
	}
	return result
}

// FilterAIHistoryByDatasources drops rows whose datasource is not in the
// allowed set. Pass nil to disable filtering.
func FilterAIHistoryByDatasources(rows []pkgmetadata.AIQueryHistoryEntry, allowed map[string]struct{}) []pkgmetadata.AIQueryHistoryEntry {
	if allowed == nil {
		return rows
	}
	result := rows[:0]
	for _, row := range rows {
		if _, ok := allowed[row.DatasourceID]; ok {
			result = append(result, row)
		}
	}
	return result
}

// FilterQueryHistoryByDatasources drops rows whose datasource is not in the
// allowed set. Pass nil to disable filtering.
func FilterQueryHistoryByDatasources(rows []pkgquery.HistoryEntry, allowed map[string]struct{}) []pkgquery.HistoryEntry {
	if allowed == nil {
		return rows
	}
	result := rows[:0]
	for _, row := range rows {
		if _, ok := allowed[row.DatasourceID]; ok {
			result = append(result, row)
		}
	}
	return result
}

// MaskAIHistoryRow zeroes the heavy/sensitive payload fields on a single row.
// Used when surfacing other users' rows to admins-but-not-super-admins in
// summary contexts, or when the caller's permission does not include
// ai:queue:view_details but the row remains visible for some auxiliary reason.
func MaskAIHistoryRow(row *pkgmetadata.AIQueryHistoryEntry) {
	if row == nil {
		return
	}
	row.Question = ""
	row.PromptContext = nil
	row.AIResponse = nil
	row.LogicalQuery = nil
	row.Warnings = nil
}
