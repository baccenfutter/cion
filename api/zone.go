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
	"sync"
	"time"

	my_middleware "github.com/baccenfutter/cion/middleware"
	"github.com/labstack/echo"
	"github.com/satori/go.uuid"
	"golang.org/x/time/rate"
)

type (
	// zone is a container for zone registration requests/responses.
	zone struct {
		Zone    string `json:"zone"`
		AuthKey string `json:"auth_key"`
	}

	aRecordParams struct {
		Name string `json:"name" form:"name" query:"name"`
		Addr string `json:"address" form:"address" query:"address"`
	}

	mxRecordParams struct {
		Pref string `json:"pref" form:"pref" query:"pref"`
		Name string `json:"name" form:"name" query:"name"`
	}

	// srvRecordParams is a container for the record update requests/responses.
	srvRecordParams struct {
		Srv    string `json:"srv" form:"srv" query:"srv"`
		Proto  string `json:"proto" form:"proto" query:"proto"`
		Prio   uint16 `json:"prio" form:"prio" query:"prio"`
		Weight uint16 `json:"weight" form:"weight" query:"weight"`
		Port   uint16 `json:"port" form:"port" query:"port"`
		Name   string `json:"name" form:"name" query:"name"`
	}

	txtRecordParams struct {
		Value string `json:"value" form:"value" query:"value" required:"false"`
	}

	cnameRecordParams struct {
		Name string `json:"name" form:"name" query:"name"`
		Dest string `json:"dest" form:"dest" query:"dest"`
	}

	limiter struct {
		Lock    sync.Mutex
		Limiter *rate.Limiter
	}
)

var (
	// Some regular expressions for field validation.
	validARecord  = regexp.MustCompile(`^([a-zA-Z0-9\-]+[a-zA-Z0-9]*){1,61}$`)
	validService  = regexp.MustCompile(`^([a-zA-Z0-9]+[a-zA-Z0-9\-]*){1,61}$`)
	validProto    = regexp.MustCompile(`^([a-zA-Z0-9]*){1,16}$`)
	validHostname = regexp.MustCompile(`^([a-zA-Z0-9_\-\.]*){4,253}$`)
	validIPv4     = regexp.MustCompile(`^(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$`)

	// rate-limit
	limitMutexRegister sync.Mutex
	limitRegistrations = map[string]time.Time{}
	limitMutexUpdates  sync.Mutex
	limitUpdates       = map[string]*limiter{}
)

var configTemplate = `zone "{{ .ZoneFQDN }}" IN {
    type master;
    file "{{ .ZoneFile }}";
    allow-update { rndc-key; };
};
`

func (aParams aRecordParams) isValid() bool {
	if aParams.Name == "" {
		return false
	}
	if !validARecord.MatchString(aParams.Name) {
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

func (srvParams srvRecordParams) isValid() bool {
	if srvParams.Srv == "" {
		return false
	}
	if !validService.MatchString(srvParams.Srv) {
		return false
	}
	if srvParams.Proto == "" {
		return false
	}
	if !validProto.MatchString(srvParams.Proto) {
		return false
	}
	if srvParams.Name == "" {
		return false
	}
	if !validHostname.MatchString(srvParams.Name) {
		return false
	}
	return true
}

func (mxParams mxRecordParams) isValid() bool {
	if mxParams.Name == "" {
		return false
	}
	return true
}

func (txtParams txtRecordParams) isValid() bool {
	if txtParams.Value == "" {
		return false
	}
	return true
}

func (cnameParams cnameRecordParams) isValid() bool {
	if cnameParams.Name == "" {
		return false
	}
	if !validHostname.MatchString(cnameParams.Name) {
		return false
	}
	if cnameParams.Dest == "" {
		return false
	} else if cnameParams.Dest[len(cnameParams.Dest)-1] == '.' {
		// Remove trailing dot if present
		cnameParams.Dest = cnameParams.Dest[0 : len(cnameParams.Dest)-1]
	}
	if !validHostname.MatchString(cnameParams.Dest) {
		return false
	}
	return true
}

// createZone is the echo handler for registering a zone.
// It returns
// - http202 and an auth_key if the zone was registered successfully
// - http423 if the zone is already taken
// - http429 if more than one zone is registered by the same IP within 24h
func createZone(c echo.Context) error {
	remote := c.Request().RemoteAddr
	parts := strings.Split(remote, ":")
	addr := strings.Join(parts[:len(parts)-1], ":")

	limitMutexRegister.Lock()
	defer limitMutexRegister.Unlock()

	lastRegistration, ok := limitRegistrations[addr]
	if ok {
		if time.Since(lastRegistration) < time.Duration(time.Hour*24) {
			log.Printf("warning: registration limit reached for: %s\n", addr)
			return echo.NewHTTPError(
				429,
				fmt.Sprintf(
					"next registration is possible in %s",
					time.Duration(time.Hour*24)-time.Since(lastRegistration),
				),
			)
		}
	}

	zone := new(zone)
	err := c.Bind(zone)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "request parameters malformed")
	}

	// If a corresponding key-file already exists, the zone is not available.
	filePath := filepath.Join(CionKeyDir, zone.Zone+".key")
	_, err = os.Stat(filePath)
	if err == nil {
		return echo.NewHTTPError(http.StatusLocked, "namespace already occupied")
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

	// Remember the timestamp for rate-limiting.
	limitRegistrations[addr] = time.Now()

	return c.JSON(http.StatusAccepted, zone)
}

// createUpdateOrDeleteRecord is the echo handler for adding/update records.
// It returns
// - http200 if the record was added/updated successfully
// - http400 if the request was malformed
// - http401 if the authentication failed
func createUpdateOrDeleteRecord(c echo.Context) error {
	cionHeaders := c.Get("cion_headers").(my_middleware.CionHeaders)

	limitMutexUpdates.Lock()
	key := cionHeaders.AuthKey
	l, ok := limitUpdates[key]
	if !ok {
		limitUpdates[key] = &limiter{
			Limiter: rate.NewLimiter(1, 10),
		}
		l = limitUpdates[key]
	}
	allowed := !l.Limiter.Allow()
	limitMutexUpdates.Unlock()
	if allowed {
		log.Printf("warning: client reached update limit: %s\n", c.Request().RemoteAddr)
		return echo.NewHTTPError(429, "one update per second with max burst of ten, please")
	}

	if cionHeaders.UpdateType != "" {
		if strings.ToLower(cionHeaders.UpdateType) == "a" {
			return createOrUpdateARecord(c, cionHeaders.Zone)
		} else if strings.ToLower(cionHeaders.UpdateType) == "srv" {
			return createOrUpdateSRVRecord(c, cionHeaders.Zone)
		} else if strings.ToLower(cionHeaders.UpdateType) == "mx" {
			return createOrUpdateMXRecord(c, cionHeaders.Zone)
		} else if strings.ToLower(cionHeaders.UpdateType) == "txt" {
			return createOrUpdateTXTRecord(c, cionHeaders.Zone)
		} else if strings.ToLower(cionHeaders.UpdateType) == "cname" {
			return createOrUpdateCNAMERecord(c, cionHeaders.Zone)
		}
		return echo.NewHTTPError(
			http.StatusBadRequest,
			fmt.Sprintf("invalid update type: %s", cionHeaders.UpdateType),
		)
	} else if cionHeaders.DeleteType != "" {
		if strings.ToLower(cionHeaders.DeleteType) == "a" {
			return deleteARecord(c, cionHeaders.Zone)
		} else if strings.ToLower(cionHeaders.DeleteType) == "mx" {
			return deleteMXRecord(c, cionHeaders.Zone)
		} else if strings.ToLower(cionHeaders.DeleteType) == "srv" {
			return deleteSRVRecord(c, cionHeaders.Zone)
		} else if strings.ToLower(cionHeaders.DeleteType) == "txt" {
			return deleteTXTRecord(c, cionHeaders.Zone)
		} else if strings.ToLower(cionHeaders.DeleteType) == "cname" {
			return deleteCNAMERecord(c, cionHeaders.Zone)
		}

		return echo.NewHTTPError(
			http.StatusBadRequest,
			fmt.Sprintf("invalid delete type: %s", cionHeaders.UpdateType),
		)
	}
	return echo.NewHTTPError(
		http.StatusBadRequest,
		"please specify X-Cion-Update-Type or X-Cion-Delete-Type header!",
	)
}

func getAParams(c echo.Context, zone string, delete bool) (*exec.Cmd, error) {
	aParams := new(aRecordParams)
	if err := c.Bind(aParams); err != nil {
		return nil, echo.NewHTTPError(
			http.StatusBadRequest,
			"request parameters malformed!",
		)
	}

	if !aParams.isValid() {
		return nil, echo.NewHTTPError(
			http.StatusBadRequest,
			"request parameters not valid or missing!",
		)
	}

	os.Setenv("CION_DEPLOY_UPDATE", "yes")
	if delete {
		os.Setenv("CION_DELETE_ONLY", "yes")
	}
	cmd := exec.Command(
		"cion_compile_update_a",
		zone,
		aParams.Name,
		aParams.Addr,
	)
	return cmd, nil
}

func createOrUpdateARecord(c echo.Context, zone string) error {
	cmd, err := getAParams(c, zone, false)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "err")
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, out)
	}

	return c.String(http.StatusAccepted, string(out))
}

func deleteARecord(c echo.Context, zone string) error {
	cmd, err := getAParams(c, zone, true)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, out)
	}

	return c.String(http.StatusAccepted, string(out))
}

func getMXParams(c echo.Context, zone string, delete bool) (*exec.Cmd, error) {
	mxParams := new(mxRecordParams)
	if err := c.Bind(mxParams); err != nil {
		return nil, echo.NewHTTPError(
			http.StatusBadRequest,
			"request parameters malformed!",
		)
	}

	if !mxParams.isValid() {
		return nil, echo.NewHTTPError(
			http.StatusBadRequest,
			"request parameters not valid or missing!",
		)
	}

	os.Setenv("CION_DEPLOY_UPDATE", "yes")
	if delete {
		os.Setenv("CION_DELETE_ONLY", "yes")
	}
	cmd := exec.Command(
		"cion_compile_update_mx",
		zone,
		mxParams.Pref,
		mxParams.Name,
	)

	return cmd, nil
}

func createOrUpdateMXRecord(c echo.Context, zone string) error {
	cmd, err := getMXParams(c, zone, false)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, out)
	}

	return c.String(http.StatusAccepted, string(out))
}

func deleteMXRecord(c echo.Context, zone string) error {
	cmd, err := getMXParams(c, zone, true)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, out)
	}

	return c.String(http.StatusAccepted, string(out))
}

func getSRVParams(c echo.Context, zone string, delete bool) (*exec.Cmd, error) {
	srvParams := new(srvRecordParams)
	if err := c.Bind(srvParams); err != nil {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "request parameters malformed!")
	}

	if !srvParams.isValid() {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "request parameters not valid or missing!")
	}

	os.Setenv("CION_DEPLOY_UPDATE", "yes")
	if delete {
		os.Setenv("CION_DELETE_ONLY", "yes")
	}
	cmd := exec.Command(
		"cion_compile_update_srv",
		zone,
		srvParams.Srv,
		srvParams.Proto,
		fmt.Sprintf("%d", srvParams.Prio),
		fmt.Sprintf("%d", srvParams.Weight),
		fmt.Sprintf("%d", srvParams.Port),
		srvParams.Name,
	)
	return cmd, nil
}

func createOrUpdateSRVRecord(c echo.Context, zone string) error {
	cmd, err := getSRVParams(c, zone, false)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, out)
	}

	return c.String(http.StatusAccepted, string(out))
}

func deleteSRVRecord(c echo.Context, zone string) error {
	cmd, err := getSRVParams(c, zone, true)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, out)
	}

	return c.String(http.StatusAccepted, string(out))
}

func getTXTParams(c echo.Context, zone string, delete bool) (*exec.Cmd, error) {
	txtParams := new(txtRecordParams)
	if err := c.Bind(txtParams); err != nil {
		return nil, echo.NewHTTPError(
			http.StatusBadRequest,
			"request parameters malformed!",
		)
	}

	if !txtParams.isValid() {
		return nil, echo.NewHTTPError(
			http.StatusBadRequest,
			"request parameters not valid or missing!",
		)
	}

	os.Setenv("CION_DEPLOY_UPDATE", "yes")
	if delete {
		os.Setenv("CION_DELETE_ONLY", "yes")
	}
	cmd := exec.Command(
		"cion_compile_update_txt",
		zone,
		txtParams.Value,
	)

	return cmd, nil
}

func createOrUpdateTXTRecord(c echo.Context, zone string) error {
	cmd, err := getTXTParams(c, zone, false)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, out)
	}

	return c.String(http.StatusAccepted, string(out))
}

func deleteTXTRecord(c echo.Context, zone string) error {
	cmd, err := getTXTParams(c, zone, true)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, out)
	}

	return c.String(http.StatusAccepted, string(out))
}

func getCNAMEParams(c echo.Context, zone string, delete bool) (*exec.Cmd, error) {
	cnameParams := new(cnameRecordParams)
	if err := c.Bind(cnameParams); err != nil {
		return nil, echo.NewHTTPError(
			http.StatusBadRequest,
			"request parameters malformed!",
		)
	}

	if !cnameParams.isValid() {
		return nil, echo.NewHTTPError(
			http.StatusBadRequest,
			"request parameters not valid or missing!",
		)
	}

	os.Setenv("CION_DEPLOY_UPDATE", "yes")
	cmd := exec.Command(
		"cion_compile_update_cname",
		zone,
		cnameParams.Name,
		cnameParams.Dest,
	)

	return cmd, nil
}

func createOrUpdateCNAMERecord(c echo.Context, zone string) error {
	cmd, err := getCNAMEParams(c, zone, false)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, out)
	}

	return c.String(http.StatusAccepted, string(out))
}

func deleteCNAMERecord(c echo.Context, zone string) error {
	cmd, err := getCNAMEParams(c, zone, true)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, out)
	}

	return c.String(http.StatusAccepted, string(out))
}
