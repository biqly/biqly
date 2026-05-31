package core

import "errors"

// Shared validation/error message strings — useful where an error value isn't
// appropriate (e.g. HTTP handler payloads, fmt.Errorf formatting).
const (
	MsgDatasourceIDRequired = "datasource_id is required"
	MsgModelIDRequired      = "model_id is required"
	MsgUnsupportedDriver    = "unsupported datasource type"
)

var (
	ErrModelIDRequired      = errors.New(MsgModelIDRequired)
	ErrDatasourceIDRequired = errors.New(MsgDatasourceIDRequired)
	ErrLoadSemanticModel    = errors.New("load semantic model")
	ErrLoadDatasource       = errors.New("load datasource")
	ErrLoadDriver           = errors.New("load driver")
	ErrConnection           = errors.New("connection failed")
	ErrQueryExecution       = errors.New("query execution failed")
)
