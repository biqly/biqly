package core

import "errors"

var (
	ErrModelIDRequired      = errors.New("model_id is required")
	ErrDatasourceIDRequired = errors.New("datasource_id is required")
	ErrLoadSemanticModel    = errors.New("load semantic model")
	ErrLoadDatasource       = errors.New("load datasource")
	ErrLoadDriver           = errors.New("load driver")
	ErrConnection           = errors.New("connection failed")
	ErrQueryExecution       = errors.New("query execution failed")
)
