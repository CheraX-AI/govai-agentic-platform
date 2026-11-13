package wiring

// Rate limits per client IP and per submitted identifier (email or NIC) are set
// where each limiter is built. Counters are per process and reset on restart.
// The attempt counts can be changed in .env (*_RATE_LIMIT, see config.RateLimits);
// the time windows stay fixed:
//
//	login                5 per 15 minutes      LOGIN_RATE_LIMIT
//	refresh/session      60 per minute         REFRESH_RATE_LIMIT
//	registration per IP  5 per 15 minutes      REGISTRATION_RATE_LIMIT_IP
//	registration email   3 per hour            REGISTRATION_RATE_LIMIT_EMAIL

// orDefault returns n when it is set (above zero), otherwise the default.
func orDefault(n, def int) int {
	if n > 0 {
		return n
	}

	return def
}
