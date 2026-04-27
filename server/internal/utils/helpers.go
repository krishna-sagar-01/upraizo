package utils

import "github.com/google/uuid"

// StringPtr returns a pointer to the passed string.
func StringPtr(s string) *string {
	return &s
}

// UUIDPtr returns a pointer to the passed UUID.
func UUIDPtr(id uuid.UUID) *uuid.UUID {
	return &id
}

// Int64Ptr returns a pointer to the passed int64.
func Int64Ptr(i int64) *int64 {
	return &i
}

// BoolPtr returns a pointer to the passed bool.
func BoolPtr(b bool) *bool {
	return &b
}
