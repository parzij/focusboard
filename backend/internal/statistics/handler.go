package statistics

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/parzij/focusboard/backend/internal/httpx"
)

type Handler struct{ service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

var ianaName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_+-]*(/[A-Za-z][A-Za-z0-9_+-]*)*$`)

func queryParameters(raw string) (Period, *time.Location, map[string]string) {
	values, err := url.ParseQuery(raw)
	if err != nil {
		return "", nil, map[string]string{"query": "Enter valid query parameters."}
	}
	fields := make(map[string]string)
	for name, entries := range values {
		switch {
		case name != "period" && name != "timezone":
			fields["query"] = "Only period and timezone are allowed."
		case len(entries) != 1:
			fields[name] = "Provide this parameter once."
		}
	}

	period := Days
	if values.Has("period") {
		period = Period(values.Get("period"))
		if period != Days && period != Weeks && fields["period"] == "" {
			fields["period"] = "Choose days or weeks."
		}
	}

	location := time.UTC
	if values.Has("timezone") && fields["timezone"] == "" {
		name := values.Get("timezone")
		root, _, _ := strings.Cut(name, "/")
		switch {
		case !ianaName.MatchString(name), name == "Local", name == "localtime", name == "posixrules", root == "posix", root == "right":
			location = nil
		default:
			location, err = time.LoadLocation(name)
		}
		if err != nil || location == nil {
			fields["timezone"] = "Enter a valid IANA timezone."
		}
	}
	return period, location, fields
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	period, timezone, fields := queryParameters(r.URL.RawQuery)
	if len(fields) != 0 {
		httpx.ValidationError(w, fields)
		return
	}
	report, err := h.service.Get(r.Context(), httpx.UserID(r.Context()), period, timezone)
	if err != nil {
		httpx.InternalError(w, r, "statistics request", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, public(report))
}
