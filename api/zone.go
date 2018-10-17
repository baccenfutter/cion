package api

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	"github.com/c-base/cion/config"
	my_middleware "github.com/c-base/cion/middleware"
	"github.com/labstack/echo"
	"github.com/satori/go.uuid"
)

type (
	// zone is a container for zone registration requests/responses.
	zone struct {
		Zone    string `json:"zone"`
		AuthKey string `json:"auth_key"`
	}

	// recordParams is a container for the record update requests/responses.
	recordParams struct {
		Srv    string `json:"srv" form:"srv" query:"srv"`
		Proto  string `json:"proto" form:"proto" query:"proto"`
		Prio   uint16 `json:"prio" form:"prio" query:"prio"`
		Weight uint16 `json:"weight" form:"weight" query:"weight"`
		Port   uint16 `json:"port" form:"port" query:"port"`
		Dest   string `json:"dest" form:"dest" query:"dest"`
	}
)

var (
	// Some regular expressions for field validation.
	validService     = regexp.MustCompile(`^[a-zA-Z0-9]+?[a-zA-Z0-9\-]{1,61}$`)
	validProto       = regexp.MustCompile(`^[a-zA-Z0-9]{1,16}$`)
	validDestination = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]{4,253}$`)
)

var configTemplate = `zone "{{ .ZoneFQDN }}" IN {
    type master;
    file "{{ .ZoneFile }}";
    allow-update { rndc-key; };
};
`

// isValid returns true if the recordParams validate.
func (params recordParams) isValid() bool {
	if params.Srv == "" {
		return false
	}
	if !validService.MatchString(params.Srv) {
		return false
	}
	if params.Proto == "" {
		return false
	}
	if !validProto.MatchString(params.Proto) {
		return false
	}
	if params.Dest == "" {
		return false
	}
	if !validDestination.MatchString(params.Dest) {
		return false
	}
	return true
}

// createZone is the echo handler for registering a zone.
// It returns
// - http423 if the zone is already taken
// - http202 and an auth_key if the zone was registered successfully
func createZone(c echo.Context) error {
	zone := new(zone)
	err := c.Bind(zone)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "request parameters malformed!")
	}

	// If a corresponding key-file already exists, the zone is not available.
	filePath := filepath.Join(CionKeyDir, zone.Zone+".key")
	_, err = os.Stat(filePath)
	if err == nil {
		return echo.NewHTTPError(http.StatusLocked, "")
	}

	// Generate a unique authentication key via sha256(uuid4())
	uuid, err := uuid.NewV4()
	if err != nil {
		return err
	}
	h := sha256.New()
	h.Write(uuid.Bytes())
	key := hex.EncodeToString(h.Sum(nil))

	// Save the key to disk, "persisting the account".
	ioutil.WriteFile(filePath, []byte(key), os.FileMode(0600))

	// Run the cion_compile_config script
	out, err := exec.Command("cion_compile_config").Output()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, out)
	}

	// Add auth-key to response and return it.
	zone.AuthKey = key
	return c.JSON(http.StatusAccepted, zone)
}

// createOrUpdateRecord is the echo handler for adding/update records.
// It returns
// - http401 if the authentication failed
// - http400 if the request was malformed
// - http200 if the record was added/updated successfully
func createOrUpdateRecord(c echo.Context) error {
	params := new(recordParams)
	if err := c.Bind(params); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "request parameters malformed!")
	}

	if !params.isValid() {
		log.Println(params)
		return echo.NewHTTPError(http.StatusBadRequest, "request parameters not valid or missing!")
	}

	cionHeaders := c.Get("cion_headers").(my_middleware.CionHeaders)
	os.Setenv("CION_DEPLOY_UPDATE", "yes")
	cmd := exec.Command(
		"cion_compile_update",
		fmt.Sprintf("%s.%s", cionHeaders.Zone, config.Config().RootDomain),
		params.Srv,
		params.Proto,
		fmt.Sprintf("%d", params.Prio),
		fmt.Sprintf("%d", params.Weight),
		fmt.Sprintf("%d", params.Port),
		params.Dest,
	)

	out, err := cmd.Output()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, cmd)
	}

	return c.String(http.StatusAccepted, string(out))
}

func deleteRecord(c echo.Context) error { return nil }
