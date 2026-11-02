package service

import (
	"errors"
	"fmt"
)

type ErrorKind string

const (
	KindValidation ErrorKind = "validation"
	KindNotFound   ErrorKind = "not_found"
	KindConflict   ErrorKind = "conflict"
	KindInternal   ErrorKind = "internal"
)

type Error struct {
	Kind    ErrorKind
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Cause)
	}
	return e.Message
}

func (e *Error) Unwrap() error { return e.Cause }

func validation(message string) error {
	return &Error{Kind: KindValidation, Code: "VALIDATION_ERROR", Message: message}
}

func notFound(message string) error {
	return &Error{Kind: KindNotFound, Code: "RESOURCE_NOT_FOUND", Message: message}
}

func conflict(code, message string) error {
	return &Error{Kind: KindConflict, Code: code, Message: message}
}

func internal(message string, cause error) error {
	return &Error{Kind: KindInternal, Code: "INTERNAL_ERROR", Message: message, Cause: cause}
}

func KindOf(err error) ErrorKind {
	var serviceErr *Error
	if errors.As(err, &serviceErr) {
		return serviceErr.Kind
	}
	return KindInternal
}

func CodeOf(err error) string {
	var serviceErr *Error
	if errors.As(err, &serviceErr) && serviceErr.Code != "" {
		return serviceErr.Code
	}
	return "INTERNAL_ERROR"
}

func MessageOf(err error) string {
	var serviceErr *Error
	if errors.As(err, &serviceErr) {
		return serviceErr.Message
	}
	return "服务暂时不可用"
}
