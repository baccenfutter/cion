package api

import (
	"io/ioutil"
	"log"
	"net/http"

	"github.com/labstack/echo"
)

func landingpage(c echo.Context) error {
	page, err := ioutil.ReadFile("/public/index.html")
	if err != nil {
		log.Println("Can not load: /public/index.html")
		return echo.NewHTTPError(http.StatusInternalServerError)
	}
	return c.HTMLBlob(http.StatusOK, page)
}
