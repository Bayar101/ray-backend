package httpx

import (
	"errors"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

func validationErrors(err error) map[string]string {
	out := map[string]string{}
	var ve validator.ValidationErrors
	if errors.As(err, &ve) {
		for _, fe := range ve {
			switch fe.Tag() {
			case "required":
				out[fe.Field()] = fe.Field() + " is required"
			case "min":
				out[fe.Field()] = fe.Field() + " must be at least " + fe.Param() + " characters long"
			case "max":
				out[fe.Field()] = fe.Field() + " must be at most " + fe.Param() + " characters long"
			case "email":
				out[fe.Field()] = fe.Field() + " must be a valid email address"
			case "url":
				out[fe.Field()] = fe.Field() + " must be a valid URL"
			default:
				out[fe.Field()] = "invalid" + fe.Field()
			}
		}
	}
	return out
}
