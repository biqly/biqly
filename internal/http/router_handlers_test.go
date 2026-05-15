package http

import (
	"net/http"

	"github.com/biqly/biqly/internal/http/handlers"
)

var (
	_ interface {
		Compile(http.ResponseWriter, *http.Request)
		Run(http.ResponseWriter, *http.Request)
		Explain(http.ResponseWriter, *http.Request)
		History(http.ResponseWriter, *http.Request)
		GetHistory(http.ResponseWriter, *http.Request)
	} = (*handlers.QueryHandler)(nil)

	_ interface {
		Query(http.ResponseWriter, *http.Request)
		Preview(http.ResponseWriter, *http.Request)
		Run(http.ResponseWriter, *http.Request)
	} = (*handlers.AIHandler)(nil)

	_ interface {
		Create(http.ResponseWriter, *http.Request)
		List(http.ResponseWriter, *http.Request)
		Get(http.ResponseWriter, *http.Request)
		Delete(http.ResponseWriter, *http.Request)
		Test(http.ResponseWriter, *http.Request)
		SyncMetadata(http.ResponseWriter, *http.Request)
	} = (*handlers.DatasourceHandler)(nil)
)
