package handler

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gin-gonic/gin"
)

type DependencyCheck func(context.Context) error

type HealthHandler struct {
	Service string
	Checks  map[string]DependencyCheck
}

func (h HealthHandler) Check(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	statusCode := http.StatusOK
	deps := make(map[string]string, len(h.Checks))
	for name, check := range h.Checks {
		if err := check(ctx); err != nil {
			deps[name] = "unreachable"
			statusCode = http.StatusServiceUnavailable
			continue
		}
		deps[name] = "ok"
	}

	status := "ok"
	if statusCode != http.StatusOK {
		status = "degraded"
	}
	c.JSON(statusCode, gin.H{"status": status, "service": h.Service, "dependencies": deps})
}

func TCPURLCheck(rawURL string) DependencyCheck {
	return func(ctx context.Context) error {
		parsed, err := url.Parse(rawURL)
		if err != nil {
			return err
		}
		dialer := net.Dialer{}
		conn, err := dialer.DialContext(ctx, "tcp", parsed.Host)
		if err != nil {
			return err
		}
		return conn.Close()
	}
}
