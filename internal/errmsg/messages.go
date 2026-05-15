package errmsg

import "fmt"

const (
	UnknownDimension = "unknown dimension"
	UnknownMetric    = "unknown metric"
	UnknownField     = "unknown field"

	PermissionPolicyUnknownField = "permission policy references unknown field"
	PermissionRowFilterUnknownField = "permission row filter references unknown field"
	RowFilterUnknownFieldPrefix    = "row filter references unknown field"

	DimensionUnknownColumn          = "dimension references unknown column"
	MetricExpressionUnknownColumn   = "metric expression references unknown column"
	JoinUnknownFromColumn           = "join references unknown from column"
	JoinUnknownToColumn             = "join references unknown to column"
)

func UnknownDimensionMsg(name string) string {
	return fmt.Sprintf("%s: %s", UnknownDimension, name)
}

func UnknownMetricMsg(name string) string {
	return fmt.Sprintf("%s: %s", UnknownMetric, name)
}

func UnknownFieldMsg(name string) string {
	return fmt.Sprintf("%s: %s", UnknownField, name)
}

func ErrUnknownDimension(name string) error {
	return fmt.Errorf("%s: %s", UnknownDimension, name)
}

func ErrUnknownMetric(name string) error {
	return fmt.Errorf("%s: %s", UnknownMetric, name)
}

func ErrUnknownField(name string) error {
	return fmt.Errorf("%s: %s", UnknownField, name)
}

func RowFilterUnknownField(name string) error {
	return fmt.Errorf("%s: %s", RowFilterUnknownFieldPrefix, name)
}
