package api

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	my_middleware "github.com/baccenfutter/cion/middleware"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
)

var (
	// CionKeyDir holds the path where all keys are stored.
	CionKeyDir = "/etc/bind/keys"
)

// LoadKeys loads all keys from disk to memory.
func LoadKeys() {
	file, err := ioutil.TempFile(CionKeyDir, ".tmp")
	defer os.Remove(file.Name())

	if err != nil {
		log.Fatal(err)
	}

	out, err := exec.Command("cion_list_keys").Output()
	if err != nil {
		log.Fatal(err)
	}

	var i int
	var line string
	for i, line = range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		pairs := strings.Split(line, " ")
		if len(pairs) != 2 {
			log.Fatal("can not read key:", line)
		}
	}

	log.Printf("Loaded %d keys.\n", i)
}

// ListenAndServe starts and runs the HTTP server.
func ListenAndServe() {
	e := echo.New()
	e.Static("/static", "/public/static")

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/", landingpage)
	e.File("/favicon.ico", "/public/favicon.ico")
	e.GET("/downloads/cion-tool.sh", func(c echo.Context) error {
		return c.Attachment("/public/cion-tool.sh", "cion-tool.sh")
	})
	e.PUT("/register", createZone)

	g := e.Group("/zone",
		my_middleware.Cion(),
		my_middleware.Version(),
	)
	g.POST("/:zone", createUpdateOrDeleteRecord)
	g.GET("/:zone", getRecordList)

	e.Logger.Fatal(e.Start(":80"))
}
