package domain

import "errors"

var (
	ErrNotFound         = errors.New("not found")
	ErrMaxTurnsExceeded = errors.New("max turns exceeded")
	ErrToolNotSupported = errors.New("tool not supported")
	ErrCommandBlocked   = errors.New("command blocked by security policy")
	ErrPathBlocked      = errors.New("path blocked by security policy")
	ErrUserDenied       = errors.New("user denied tool execution")
)
