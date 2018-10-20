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
	"strings"

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

	aRecordParams struct {
		Hostname string `json:"hostname" form:"hostname" query:"hostname"`
		Addr     string `json:"address" form:"address" query:"address"`
	}

	// srvRecordParams is a container for the record update requests/responses.
	srvRecordParams struct {
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
	validService  = regexp.MustCompile(`^[a-zA-Z0-9]+?[a-zA-Z0-9\-]{1,61}$`)
	validProto    = regexp.MustCompile(`^[a-zA-Z0-9]{1,16}$`)
	validHostname = regexp.MustCompile(`^[a-zA-Z0-9_\-\.]{4,253}$`)
	validIPv4     = regexp.MustCompile(`^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`)
)

var configTemplate = `zone "{{ .ZoneFQDN }}" IN {
    type master;
    file "{{ .ZoneFile }}";
    allow-update { rndc-key; };
};
`

// isValid returns true if the srvRecordParams validate.
func (aParams srvRecordParams) isValid() bool {
	if aParams.Srv == "" {
		return false
	}
	if !validService.MatchString(aParams.Srv) {
		return false
	}
	if aParams.Proto == "" {
		return false
	}
	if !validProto.MatchString(aParams.Proto) {
		return false
	}
	if aParams.Dest == "" {
		return false
	}
	if !validHostname.MatchString(aParams.Dest) {
		return false
	}
	return true
}

func (aParams aRecordParams) isValid() bool {
	if aParams.Hostname == "" {
		return false
	}
	if !validHostname.MatchString(aParams.Hostname) {
		return false
	}
	if aParams.Addr == "" {
		return false
	}
	if !validIPv4.MatchString(aParams.Addr) {
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
	exec.Command(fmt.Sprintf("chown named. %s", filePath))

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
	cionHeaders := c.Get("cion_headers").(my_middleware.CionHeaders)

	if cionHeaders.UpdateType == "" {
		return echo.NewHTTPError(
			http.StatusBadRequest,
			"Please specify X-Cion-Update-Type header!",
		)
	}

	if strings.ToLower(cionHeaders.UpdateType) == "a" {
		return createOrUpdateARecord(c, cionHeaders.Zone)
	} else if strings.ToLower(cionHeaders.UpdateType) == "srv" {
		return createOrUpdateSrvRecord(c, cionHeaders.Zone)
	}
	return echo.NewHTTPError(
		http.StatusBadRequest,
		fmt.Sprintf("Invalid update type: %s", cionHeaders.UpdateType),
	)
}

func createOrUpdateARecord(c echo.Context, zone string) error {
	aParams := new(aRecordParams)
	if err := c.Bind(aParams); err != nil {
		return echo.NewHTTPError(
			http.StatusBadRequest,
			"request parameters malformed!",
		)
	}

	if !aParams.isValid() {
		log.Println(aParams)
		return echo.NewHTTPError(
			http.StatusBadRequest,
			"request parameters not valid or missing!",
		)
	}

	os.Setenv("CION_DEPLOY_UPDATE", "yes")
	cmd := exec.Command(
		"cion_compile_update_a",
		zone,
		aParams.Hostname,
		aParams.Addr,
	)

	out, err := cmd.Output()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, cmd)
	}

	return c.String(http.StatusAccepted, string(out))
}

func createOrUpdateSrvRecord(c echo.Context, zone string) error {
	srvParams := new(srvRecordParams)
	if err := c.Bind(srvParams); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "request parameters malformed!")
	}

	if !srvParams.isValid() {
		log.Println(srvParams)
		return echo.NewHTTPError(http.StatusBadRequest, "request parameters not valid or missing!")
	}

	os.Setenv("CION_DEPLOY_UPDATE", "yes")
	cmd := exec.Command(
		"cion_compile_update_srv",
		zone,
		srvParams.Srv,
		srvParams.Proto,
		fmt.Sprintf("%d", srvParams.Prio),
		fmt.Sprintf("%d", srvParams.Weight),
		fmt.Sprintf("%d", srvParams.Port),
		srvParams.Dest,
	)

	out, err := cmd.Output()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, cmd)
	}

	return c.String(http.StatusAccepted, string(out))
}
