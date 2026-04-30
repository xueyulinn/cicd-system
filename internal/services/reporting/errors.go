package reporting

import (
	"errors"
	"fmt"
	"net/http"
)

var (
	errInvalidReportQuery = errors.New("invalid report query")
	errReportNotFound     = errors.New("report not found")
)

func invalidReportQuery(message string) error {
	return fmt.Errorf("%w: %s", errInvalidReportQuery, message)
}

func reportNotFound(message string) error {
	return fmt.Errorf("%w: %s", errReportNotFound, message)
}

func classifyError(err error) (int, string, string) {
	switch {
	case errors.Is(err, errInvalidReportQuery):
		return http.StatusBadRequest, "invalid_argument", err.Error()
	case errors.Is(err, errReportNotFound):
		return http.StatusNotFound, "report_not_found", err.Error()
	default:
		return http.StatusInternalServerError, "internal_error", "internal server error"
	}
}
