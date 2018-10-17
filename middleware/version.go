package middleware

import (
	"log"
	"net/http"
	"strings"

	"github.com/blang/semver"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

type (
	// VersionConfig defines the config for Version middleware.
	VersionConfig struct {
		// Skipper defines a function to skip middleware.
		Skipper middleware.Skipper

		// AllowedVersionPattern defines a semver.Range.
		AllowedVersionPattern string
	}
)

var (
	defaultRange = ">=1.0.0 <1.1.0"

	// DefaultVersionConfig is the default VersionConfig middleware config.
	DefaultVersionConfig = VersionConfig{
		Skipper:               middleware.DefaultSkipper,
		AllowedVersionPattern: defaultRange,
	}
)

// VersionWithConfig returns a Version middleware with config.
func VersionWithConfig(config VersionConfig) echo.MiddlewareFunc {
	if config.Skipper == nil {
		config.Skipper = DefaultVersionConfig.Skipper
	}

	if config.AllowedVersionPattern == "" {
		config.AllowedVersionPattern = defaultRange
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if config.Skipper(c) {
				return next(c)
			}

			acceptHeader := parseAcceptHeader(
				c.Request().Header.Get("accept"))
			if acceptHeader == nil {
				return echo.NewHTTPError(http.StatusNotAcceptable, "Please state the API version!")
			}

			version, ok := acceptHeader["version"]
			if !ok {
				return echo.NewHTTPError(http.StatusNotAcceptable, "Please state the API version!")
			}

			v, err := semver.Make(version)
			if err != nil {
				return echo.NewHTTPError(http.StatusNotAcceptable, "Can not parse version string!")
			}

			Range, err := semver.ParseRange(config.AllowedVersionPattern)
			if err != nil {
				log.Fatal("Can not parse Version.AllowedVersionPattern")
				return echo.NewHTTPError(http.StatusInternalServerError, "internal error")
			}
			if !Range(v) {
				return c.String(
					http.StatusNotAcceptable,
					"{\"error\": \"Allowed versions are: "+config.AllowedVersionPattern+"\"}",
				)
			}

			c.Set("version", version)
			log.Println("Client API version:", version)

			return next(c)
		}
	}
}

// Version returns a Version middleware with default config.
func Version() echo.MiddlewareFunc {
	return VersionWithConfig(DefaultVersionConfig)
}

func parseAcceptHeader(acceptHeader string) map[string]string {
	out := map[string]string{}
	parts := strings.Split(acceptHeader, ";")
	if len(parts) == 1 {
		return nil
	}
	parts = parts[1:]
	for _, part := range parts {
		key := strings.SplitN(part, "=", 2)[0]
		value := strings.SplitN(part, "=", 2)[1]
		out[strings.Trim(strings.ToLower(key), " ")] = strings.Trim(strings.ToLower(value), " ")
	}
	return out
}
