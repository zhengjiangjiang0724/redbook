package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	loginAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "redbook_login_attempts_total",
		Help: "Number of login attempts grouped by status.",
	}, []string{"status"})

	refreshRotations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "redbook_refresh_rotations_total",
		Help: "Number of refresh rotations grouped by status.",
	}, []string{"status"})

	logoutEvents = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "redbook_logout_events_total",
		Help: "Number of logout attempts grouped by status.",
	}, []string{"status"})

	rateLimitHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "redbook_rate_limit_hits_total",
		Help: "Rate limiter activations grouped by limiter name.",
	}, []string{"limiter"})
)

// IncLogin increments the login counter.
func IncLogin(status string) {
	loginAttempts.WithLabelValues(status).Inc()
}

// IncRefresh increments the refresh rotation counter.
func IncRefresh(status string) {
	refreshRotations.WithLabelValues(status).Inc()
}

// IncLogout increments the logout counter.
func IncLogout(status string) {
	logoutEvents.WithLabelValues(status).Inc()
}

// IncRateLimit increments the rate-limit hit counter.
func IncRateLimit(name string) {
	rateLimitHits.WithLabelValues(name).Inc()
}
