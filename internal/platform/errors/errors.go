package errors

import (
	"fmt"
	"net/http"
)

type AppError struct {
	HTTPStatus int
	Code       string
	I18nKey    string
	Args       []any
	Err        error
	Details    map[string]any
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.I18nKey, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.I18nKey)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func New(httpStatus int, code string, i18nKey string, args ...any) *AppError {
	return &AppError{
		HTTPStatus: httpStatus,
		Code:       code,
		I18nKey:    i18nKey,
		Args:       args,
	}
}

func Wrap(err error, httpStatus int, code string, i18nKey string, args ...any) *AppError {
	return &AppError{
		HTTPStatus: httpStatus,
		Code:       code,
		I18nKey:    i18nKey,
		Args:       args,
		Err:        err,
	}
}

func BadRequest(code string, i18nKey string, args ...any) *AppError {
	return New(http.StatusBadRequest, code, i18nKey, args...)
}

func NotFound(code string, i18nKey string, args ...any) *AppError {
	return New(http.StatusNotFound, code, i18nKey, args...)
}

func Internal(code string, i18nKey string, args ...any) *AppError {
	return New(http.StatusInternalServerError, code, i18nKey, args...)
}

func Unauthorized(code string, i18nKey string, args ...any) *AppError {
	return New(http.StatusUnauthorized, code, i18nKey, args...)
}

func AsAppError(err error) (*AppError, bool) {
	if err == nil {
		return nil, false
	}
	if appErr, ok := err.(*AppError); ok {
		return appErr, true
	}
	return nil, false
}
