package api

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo"
)

func getDebug(c echo.Context) error {
	return c.String(http.StatusOK, fmt.Sprintf("%s", c.Get("cion_header")))
}
