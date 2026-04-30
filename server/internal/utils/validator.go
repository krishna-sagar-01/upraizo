package utils

import (
	"fmt"
	"net/url"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/shopspring/decimal"
)

// Global validator instance (Thread-safe)
var validate = validator.New()

// ─── Regex Patterns (Compiled Once For Speed) ────────────────────────────────

var (
	// nameRegex: Only alphabets and spaces, 2 to 50 characters
	nameRegex = regexp.MustCompile(`^[a-zA-Z\s]{2,50}$`)

	// Password individual constraints (Go's RE2 doesn't support lookaheads)
	hasLower   = regexp.MustCompile(`[a-z]`)
	hasUpper   = regexp.MustCompile(`[A-Z]`)
	hasNumber  = regexp.MustCompile(`[0-9]`)
	hasSpecial = regexp.MustCompile(`[!@#$%^&*]`)
)

func init() {
	// Registering 'secure_password' tag for use in struct tags
	validate.RegisterValidation("secure_password", func(fl validator.FieldLevel) bool {
		return IsStrongPassword(fl.Field().String())
	})

	// Decimal GTE (Greater than or equal)
	validate.RegisterValidation("d_gte", func(fl validator.FieldLevel) bool {
		d, ok := fl.Field().Interface().(decimal.Decimal)
		if !ok {
			return false
		}
		param := fl.Param()
		if param == "" {
			return d.GreaterThanOrEqual(decimal.Zero)
		}
		p, _ := decimal.NewFromString(param)
		return d.GreaterThanOrEqual(p)
	})
	
	// Decimal GT Field (Greater than another field)
	validate.RegisterValidation("d_gt_field", func(fl validator.FieldLevel) bool {
		field := fl.Field()
		if field.Kind() == reflect.Ptr {
			if field.IsNil() {
				return true
			}
			field = field.Elem()
		}

		d, ok := field.Interface().(decimal.Decimal)
		if !ok {
			return false
		}

		param := fl.Param()
		parent := fl.Parent()
		
		// Handle if parent is a pointer
		if parent.Kind() == reflect.Ptr {
			parent = parent.Elem()
		}

		otherField := parent.FieldByName(param)
		if !otherField.IsValid() {
			return false
		}

		if otherField.Kind() == reflect.Ptr {
			if otherField.IsNil() {
				return true
			}
			otherField = otherField.Elem()
		}

		otherD, ok := otherField.Interface().(decimal.Decimal)
		if !ok {
			return false
		}

		return d.GreaterThan(otherD)
	})
}

// ─── Struct Validation Helper ────────────────────────────────────────────────

// ValidateStruct parses struct tags and returns a beautiful JSON-ready error map.
func ValidateStruct(s interface{}) *AppError {
	err := validate.Struct(s)
	if err != nil {
		fields := make(map[string]string)

		// Type assertion to access specific validation errors
		for _, err := range err.(validator.ValidationErrors) {
			field := strings.ToLower(err.Field())

			switch err.Tag() {
			case "required":
				fields[field] = "This field is required"
			case "email":
				fields[field] = "Invalid email address format"
			case "secure_password":
				fields[field] = "Password must include uppercase, lowercase, number, and a special character"
			case "min":
				fields[field] = fmt.Sprintf("Must be at least %s characters", err.Param())
			case "max":
				fields[field] = fmt.Sprintf("Cannot exceed %s characters", err.Param())
			case "url":
				fields[field] = "Invalid URL format"
			case "d_gt_field":
				fields[field] = fmt.Sprintf("Must be greater than %s", strings.ToLower(err.Param()))
			default:
				fields[field] = fmt.Sprintf("Validation failed on rule: %s", err.Tag())
			}
		}
		// Returning our custom AppError with 400 Bad Request
		return Validation("Input validation failed", fields)
	}
	return nil
}

// ─── Individual Functional Helpers ───────────────────────────────────────────

// IsStrongPassword validates password complexity (Uppercase, Lowercase, Number, Special)
func IsStrongPassword(password string) bool {
	if len(password) < 8 || len(password) > 72 {
		return false
	}
	return hasLower.MatchString(password) &&
		hasUpper.MatchString(password) &&
		hasNumber.MatchString(password) &&
		hasSpecial.MatchString(password)
}

// IsValidEmail checks if the email string is a valid format
func IsValidEmail(email string) bool {
	return validate.Var(email, "required,email") == nil
}

// IsValidName checks if the name only contains letters and is of correct length
func IsValidName(name string) bool {
	return nameRegex.MatchString(name)
}

// IsValidURL checks for a valid HTTP/HTTPS URL
func IsValidURL(input string) bool {
	if input == "" {
		return false
	}
	u, err := url.ParseRequestURI(input)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// IsValidAvatar checks if the URL is a valid image link
func IsValidAvatar(avatarURL string) bool {
	if !IsValidURL(avatarURL) {
		return false
	}
	lower := strings.ToLower(avatarURL)
	return strings.HasSuffix(lower, ".jpg") || 
	       strings.HasSuffix(lower, ".jpeg") || 
	       strings.HasSuffix(lower, ".png") || 
	       strings.HasSuffix(lower, ".webp")
}