package semantic

import "fmt"

type validationSink struct {
	result PublishValidationResult
}

func newValidationSink() validationSink {
	return validationSink{result: PublishValidationResult{Valid: true}}
}

func newValidationSinkWithPromptSize(model SemanticModel) validationSink {
	s := newValidationSink()
	s.result.EstimatedPromptSize = estimatePromptSize(model)
	return s
}

func (s *validationSink) addError(format string, args ...any) {
	s.result.Errors = append(s.result.Errors, fmt.Sprintf(format, args...))
	s.result.Valid = false
}

func (s *validationSink) addWarning(format string, args ...any) {
	s.result.Warnings = append(s.result.Warnings, fmt.Sprintf(format, args...))
}

func (s *validationSink) merge(other PublishValidationResult) {
	s.result.Errors = append(s.result.Errors, other.Errors...)
	s.result.Warnings = append(s.result.Warnings, other.Warnings...)
	if !other.Valid {
		s.result.Valid = false
	}
}

func (s *validationSink) finish() PublishValidationResult {
	return s.result
}
