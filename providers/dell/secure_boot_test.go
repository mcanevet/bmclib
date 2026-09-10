package dell

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bmclibErrs "github.com/bmc-toolbox/bmclib/v2/errors"
)

// biosWithSecureBootPolicy is a minimal Bios resource with an OnReset-capable
// @Redfish.Settings block, matching a real iDRAC.
const biosWithSecureBootPolicy = `{
	"@odata.type": "#Bios.v1_1_0.Bios",
	"@odata.id": "/redfish/v1/Systems/System.Embedded.1/Bios",
	"Id": "Bios",
	"Name": "BIOS Configuration Current Settings",
	"AttributeRegistry": "BiosAttributeRegistry.v1_0_3",
	"Attributes": {
		"SecureBootPolicy": "%s"
	},
	"@Redfish.Settings": {
		"@odata.type": "#Settings.v1_3_0.Settings",
		"SettingsObject": {
			"@odata.id": "/redfish/v1/Systems/System.Embedded.1/Bios/Settings"
		},
		"SupportedApplyTimes": ["OnReset"]
	}
}`

// biosWithoutSecureBootPolicy models an older platform generation that
// doesn't expose the attribute.
const biosWithoutSecureBootPolicy = `{
	"@odata.type": "#Bios.v1_1_0.Bios",
	"@odata.id": "/redfish/v1/Systems/System.Embedded.1/Bios",
	"Id": "Bios",
	"Name": "BIOS Configuration Current Settings",
	"AttributeRegistry": "BiosAttributeRegistry.v1_0_3",
	"Attributes": {
		"BootMode": "Uefi"
	}
}`

func newSecureBootTestConn(t *testing.T, mux *http.ServeMux) *Conn {
	t.Helper()

	server := httptest.NewTLSServer(mux)
	t.Cleanup(server.Close)

	parsedURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	client := New(parsedURL.Hostname(), "", "", logr.Discard(), WithPort(parsedURL.Port()), WithUseBasicAuth(true))
	require.NoError(t, client.Open(context.Background()))
	t.Cleanup(func() { _ = client.Close(context.Background()) })

	return client
}

func TestSetSecureBootKeyManagement_EnableFromStandard(t *testing.T) {
	var settingsPatched bool
	var patchedBody string

	mux := http.NewServeMux()
	mux.HandleFunc("/redfish/v1/", endpointFunc("/serviceroot.json"))
	mux.HandleFunc("/redfish/v1/Systems", endpointFunc("/systems.json"))
	mux.HandleFunc("/redfish/v1/Systems/System.Embedded.1", endpointFunc("/systems_embedded.1.json"))
	mux.HandleFunc("/redfish/v1/Systems/System.Embedded.1/Bios", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_, _ = fmt.Fprintf(w, biosWithSecureBootPolicy, secureBootPolicyStandard)
	})
	mux.HandleFunc("/redfish/v1/Systems/System.Embedded.1/Bios/Settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case http.MethodPatch:
			settingsPatched = true
			b, _ := io.ReadAll(r.Body)
			patchedBody = string(b)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	client := newSecureBootTestConn(t, mux)

	rebootRequired, err := client.SetSecureBootKeyManagement(context.Background(), true)
	require.NoError(t, err)
	assert.True(t, rebootRequired)
	assert.True(t, settingsPatched, "expected a BIOS settings job to be scheduled")
	assert.Contains(t, patchedBody, secureBootPolicyCustom)
}

// TestSetSecureBootKeyManagement_EnableAlreadyCustom verifies SetSecureBootKeyManagement PATCHes
// unconditionally even when the attribute's currently-applied value already matches what's
// requested: currently-applied state can spuriously match while a different value is genuinely
// pending from an earlier call in the same boot cycle (confirmed live - see the doc comment on
// SetSecureBootKeyManagement), so skipping the PATCH based on currently-applied state alone would
// silently leave a stale pending value in place.
func TestSetSecureBootKeyManagement_EnableAlreadyCustom(t *testing.T) {
	var settingsPatched bool

	mux := http.NewServeMux()
	mux.HandleFunc("/redfish/v1/", endpointFunc("/serviceroot.json"))
	mux.HandleFunc("/redfish/v1/Systems", endpointFunc("/systems.json"))
	mux.HandleFunc("/redfish/v1/Systems/System.Embedded.1", endpointFunc("/systems_embedded.1.json"))
	mux.HandleFunc("/redfish/v1/Systems/System.Embedded.1/Bios", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_, _ = fmt.Fprintf(w, biosWithSecureBootPolicy, secureBootPolicyCustom)
	})
	mux.HandleFunc("/redfish/v1/Systems/System.Embedded.1/Bios/Settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case http.MethodPatch:
			settingsPatched = true
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	client := newSecureBootTestConn(t, mux)

	rebootRequired, err := client.SetSecureBootKeyManagement(context.Background(), true)
	require.NoError(t, err)
	assert.True(t, rebootRequired)
	assert.True(t, settingsPatched, "expected a BIOS settings job to be scheduled even though currently-applied state already matches")
}

func TestSetSecureBootKeyManagement_Disable(t *testing.T) {
	var settingsPatched bool
	var patchedBody string

	mux := http.NewServeMux()
	mux.HandleFunc("/redfish/v1/", endpointFunc("/serviceroot.json"))
	mux.HandleFunc("/redfish/v1/Systems", endpointFunc("/systems.json"))
	mux.HandleFunc("/redfish/v1/Systems/System.Embedded.1", endpointFunc("/systems_embedded.1.json"))
	mux.HandleFunc("/redfish/v1/Systems/System.Embedded.1/Bios", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_, _ = fmt.Fprintf(w, biosWithSecureBootPolicy, secureBootPolicyCustom)
	})
	mux.HandleFunc("/redfish/v1/Systems/System.Embedded.1/Bios/Settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case http.MethodPatch:
			settingsPatched = true
			b, _ := io.ReadAll(r.Body)
			patchedBody = string(b)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	client := newSecureBootTestConn(t, mux)

	rebootRequired, err := client.SetSecureBootKeyManagement(context.Background(), false)
	require.NoError(t, err)
	assert.True(t, rebootRequired)
	assert.True(t, settingsPatched, "expected a BIOS settings job to be scheduled")
	assert.Contains(t, patchedBody, secureBootPolicyStandard)
}

func TestSetSecureBootKeyManagement_AttributeAbsent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/redfish/v1/", endpointFunc("/serviceroot.json"))
	mux.HandleFunc("/redfish/v1/Systems", endpointFunc("/systems.json"))
	mux.HandleFunc("/redfish/v1/Systems/System.Embedded.1", endpointFunc("/systems_embedded.1.json"))
	mux.HandleFunc("/redfish/v1/Systems/System.Embedded.1/Bios", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_, _ = w.Write([]byte(biosWithoutSecureBootPolicy))
	})

	client := newSecureBootTestConn(t, mux)

	rebootRequired, err := client.SetSecureBootKeyManagement(context.Background(), true)
	require.Error(t, err)
	assert.False(t, rebootRequired)

	var unsupported *bmclibErrs.ErrUnsupportedHardware
	assert.ErrorAs(t, err, &unsupported)
}

func TestSetSecureBoot_Enable(t *testing.T) {
	var settingsPatched bool
	var patchedBody string

	mux := http.NewServeMux()
	mux.HandleFunc("/redfish/v1/", endpointFunc("/serviceroot.json"))
	mux.HandleFunc("/redfish/v1/Systems", endpointFunc("/systems.json"))
	mux.HandleFunc("/redfish/v1/Systems/System.Embedded.1", endpointFunc("/systems_embedded.1.json"))
	mux.HandleFunc("/redfish/v1/Systems/System.Embedded.1/Bios", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_, _ = fmt.Fprintf(w, biosWithSecureBootPolicy, secureBootPolicyStandard)
	})
	mux.HandleFunc("/redfish/v1/Systems/System.Embedded.1/Bios/Settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case http.MethodPatch:
			settingsPatched = true
			b, _ := io.ReadAll(r.Body)
			patchedBody = string(b)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	client := newSecureBootTestConn(t, mux)

	err := client.SetSecureBoot(context.Background(), true)
	require.NoError(t, err)
	assert.True(t, settingsPatched, "expected a BIOS settings job to be scheduled")
	assert.Contains(t, patchedBody, `"SecureBoot":"Enabled"`)
}

func TestSetSecureBoot_Disable(t *testing.T) {
	var settingsPatched bool
	var patchedBody string

	mux := http.NewServeMux()
	mux.HandleFunc("/redfish/v1/", endpointFunc("/serviceroot.json"))
	mux.HandleFunc("/redfish/v1/Systems", endpointFunc("/systems.json"))
	mux.HandleFunc("/redfish/v1/Systems/System.Embedded.1", endpointFunc("/systems_embedded.1.json"))
	mux.HandleFunc("/redfish/v1/Systems/System.Embedded.1/Bios", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		_, _ = fmt.Fprintf(w, biosWithSecureBootPolicy, secureBootPolicyStandard)
	})
	mux.HandleFunc("/redfish/v1/Systems/System.Embedded.1/Bios/Settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		case http.MethodPatch:
			settingsPatched = true
			b, _ := io.ReadAll(r.Body)
			patchedBody = string(b)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	client := newSecureBootTestConn(t, mux)

	err := client.SetSecureBoot(context.Background(), false)
	require.NoError(t, err)
	assert.True(t, settingsPatched, "expected a BIOS settings job to be scheduled")
	assert.Contains(t, patchedBody, `"SecureBoot":"Disabled"`)
}

// TestSetSecureBoot_RecoversFromPendingConflict verifies SetSecureBoot recovers from iDRAC's
// one-pending-job-at-a-time limit the same way every other Dell BIOS-backed feature does (see
// TestSetBiosConfiguration_PendingSettingsConflictRecovery), merging with whatever is already
// staged rather than failing outright - this is the exact case confirmed live: a stale pending
// job left over from an earlier, unrelated BIOS-affecting call blocked a plain SetSecureBoot
// before it was routed through setBiosConfiguration.
func TestSetSecureBoot_RecoversFromPendingConflict(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/redfish/v1/", endpointFunc("/serviceroot.json"))
	mux.HandleFunc("/redfish/v1/Systems", endpointFunc("/systems.json"))
	mux.HandleFunc("/redfish/v1/Systems/System.Embedded.1", endpointFunc("/systems_embedded.1.json"))
	mux.HandleFunc("/redfish/v1/Systems/System.Embedded.1/Bios", func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodGet, r.Method)
		_, _ = w.Write([]byte(biosWithSettingsRedirect))
	})

	var patchBodies []string
	mux.HandleFunc("/redfish/v1/Systems/System.Embedded.1/Bios/Settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_, _ = w.Write([]byte(`{"Attributes": {"HttpDev1EnDis": "Enabled"}}`))
		case http.MethodPatch:
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			patchBodies = append(patchBodies, string(body))
			if len(patchBodies) == 1 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(dellPendingSettingsCommitted))
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	var deleteCalls int
	jobServiceMux(mux, "Starting", "", &deleteCalls)

	client := newSecureBootTestConn(t, mux)

	err := client.SetSecureBoot(context.Background(), true)
	require.NoError(t, err, "expected the conflict to be recovered from, not propagated")

	assert.Equal(t, 1, deleteCalls, "expected the pending job to be deleted exactly once")
	require.Len(t, patchBodies, 2, "expected the rejected PATCH and exactly one retry")
	assert.Contains(t, patchBodies[1], `"HttpDev1EnDis":"Enabled"`, "expected the retry to preserve the already-pending attribute")
	assert.Contains(t, patchBodies[1], `"SecureBoot":"Enabled"`)
}
