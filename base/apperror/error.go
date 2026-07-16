package apperror

import "errors"

var (
	ErrConflict     = errors.New("resource conflict")
	ErrInvalidInput = errors.New("invalid input")
	ErrNotFound     = errors.New("resource not found")
	ErrUnauthorized = errors.New("unauthorized")
)
