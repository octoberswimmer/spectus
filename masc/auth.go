//go:build js && wasm

package main

import (
	"errors"
	"strings"
)

type unauthorizedError struct {
	message string
}

func (e unauthorizedError) Error() string {
	return e.message
}

func newUnauthorizedError(message string) error {
	return unauthorizedError{message: message}
}

func isUnauthorized(err error) bool {
	if err == nil {
		return false
	}
	var authErr unauthorizedError
	if errors.As(err, &authErr) {
		return true
	}
	return isUnauthorizedMessage(err.Error())
}

func isUnauthorizedMessage(message string) bool {
	msg := strings.ToLower(message)
	return strings.Contains(msg, "status 401") || strings.Contains(msg, "bad credentials")
}

func isStaleHead(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "Expected branch to point to")
}
