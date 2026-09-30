package search

import "github.com/gin-gonic/gin"

// RegisterRoutes mounts the public search surface and the reindex admin surface.
//
// adminRoleMW is passed in rather than built here, and it is deliberately NOT the
// shared roleMW from cmd/server/main.go. That gate admits "institution" and
// "scholarship_provider" alongside the platform operators, and behind it a
// paying customer could trigger a reindex that nulls every embedding column on
// the platform and re-embeds all of it. access.go has the full argument.
//
// THE ROUTE THAT MATTERED WAS NOT ON roleMW AT ALL. This file previously mounted
// the same h.Reindex handler twice: once behind adminRoleMW's predecessor, and
// once as v1.POST("/search/reindex") on the bare /api/v1 group with no
// middleware — no auth, no role, nothing. A destructive global operation, public.
// Both copies are now behind authMW and adminRoleMW.
//
// The duplicate is kept rather than removed on purpose. Nothing in the repo calls
// /api/v1/search/reindex — the only caller of either path is the superadmin
// dashboard's SettingsSection.tsx, which uses /api/v1/admin/search/reindex — so
// deleting the public spelling would be safe today. But whether the short alias
// should exist at all is a product call about the API surface, not an
// authorisation one, and gating it closes the hole without inventing that answer.
// Flagged in the report as worth removing.
//
// The service is the authority for both, so a caller who passes the wide roleMW
// back in still cannot trigger the sweep, and a handler mounted with no
// middleware at all is still refused by the first thing Reindex does.
// TestReindexGateCannotBeWidenedByTheCaller pins that.
//
// The public reads are untouched: /search, /search/suggest and
// /search/history are what students search with. /search/vector-status is left
// public as well — it is a liveness probe, and it is flagged in the report as
// disclosing that pgvector and Meilisearch are installed.
func RegisterRoutes(r *gin.Engine, authMW, roleMW, adminRoleMW gin.HandlerFunc, h *Handler) {
	if h == nil {
		return
	}

	v1 := r.Group("/api/v1")
	{
		v1.GET("/search", h.Search)
		v1.GET("/search/suggest", h.Suggest)
		v1.GET("/search/vector-status", h.GetVectorStatus)

		history := v1.Group("/search/history")
		history.Use(authMW)
		{
			history.GET("", h.GetSearchHistory)
			history.POST("", h.SaveSearchHistory)
		}
	}

	admin := v1.Group("/admin/search")
	admin.Use(authMW)
	admin.Use(adminRoleMW)
	{
		admin.POST("/reindex", h.Reindex)
		admin.GET("/reindex/status", h.ReindexStatus)
	}

	// The short alias, closed. Was mounted here with no middleware at all; see
	// the note on RegisterRoutes.
	alias := v1.Group("/search")
	alias.Use(authMW)
	alias.Use(adminRoleMW)
	{
		alias.POST("/reindex", h.Reindex)
	}
}
