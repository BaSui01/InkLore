package errors

// Contract creates a safe R1 error. Details must contain contract-defined values only.
func Contract(status int, code string, details map[string]any) *AppError {
	return &AppError{HTTPStatus: status, Code: code, I18nKey: "errors." + code, Details: details}
}
