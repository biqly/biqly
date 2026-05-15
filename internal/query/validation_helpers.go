package query

func validationErr(field, message string) error {
	return ValidationErrors{&ValidationError{Field: field, Message: message}}
}
