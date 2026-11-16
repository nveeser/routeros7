package routerosv7

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nveeser/routeros7/session"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testNamedResource struct {
	Id      string `json:".id,omitempty"`
	ResName string `json:"name"`
}

func (r testNamedResource) CollectionPath() string { return "interface/bridge" }
func (r testNamedResource) Name() string           { return r.ResName }

type emptyResource struct{}

func (emptyResource) Requests() []session.Requester { return nil }

func newTestClient(t *testing.T, handler http.HandlerFunc) (*session.Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	c, err := session.NewClient(srv.URL, session.WithHTTPClient(srv.Client()))
	require.NoError(t, err)
	return c, srv
}

func TestReadSingleton(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/rest/system/identity", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"core-router"}`))
	})

	var ident IdentityConfig
	err := Read(context.Background(), c, &ident)
	require.NoError(t, err)
	assert.Equal(t, "core-router", ident.Name)
}

func TestReadComposite(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/system/identity":
			_, _ = w.Write([]byte(`{"name":"gw-01"}`))
		case "/rest/system/clock":
			_, _ = w.Write([]byte(`{"time-zone-name":"UTC","time-zone-autodetect":"true"}`))
		case "/rest/system/ntp/client":
			_, _ = w.Write([]byte(`{"enabled":"true","mode":"unicast","servers":"1.1.1.1,8.8.8.8"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":404}`))
		}
	})

	var sys SystemConfig
	err := Read(context.Background(), c, &sys)
	require.NoError(t, err)
	assert.Equal(t, "gw-01", sys.Identity.Name)
	assert.Equal(t, "UTC", sys.Clock.TimeZoneName)
	assert.True(t, sys.Clock.TimeZoneAutodetect)
	assert.Equal(t, MBool(true), sys.NTP.Enabled)
	assert.Equal(t, "unicast", sys.NTP.Mode)
	assert.Equal(t, MList{"1.1.1.1", "8.8.8.8"}, sys.NTP.Servers)
}

func TestReadEmpty(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected request: %s", r.URL.Path)
	})

	err := Read(context.Background(), c, emptyResource{})
	require.NoError(t, err)
}

func TestReadError(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/system/clock" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`internal error`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"gw-01"}`))
	})

	var sys SystemConfig
	err := Read(context.Background(), c, &sys)
	require.Error(t, err)
}

func TestReadSlice(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/rest/ip/address", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{".id":"*1","address":"10.0.0.1/24","interface":"ether1","disabled":"false"},
			{".id":"*2","address":"10.0.1.1/24","interface":"ether2","disabled":"true"}
		]`))
	})

	var addrs []AddressConfig
	err := ReadSlice(context.Background(), c, &addrs)
	require.NoError(t, err)
	require.Len(t, addrs, 2)
	assert.Equal(t, "*1", addrs[0].Id)
	assert.Equal(t, "10.0.0.1/24", addrs[0].Address)
	assert.Equal(t, "ether1", addrs[0].Interface)
	assert.Equal(t, MBool(false), addrs[0].Disabled)
	assert.Equal(t, "*2", addrs[1].Id)
	assert.Equal(t, MBool(true), addrs[1].Disabled)
}

func TestReadId(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/rest/ip/address/*1", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{".id":"*1","address":"192.168.88.1/24","interface":"bridge"}`))
	})

	var addr AddressConfig
	err := ReadId(context.Background(), c, &addr, "*1")
	require.NoError(t, err)
	assert.Equal(t, "*1", addr.Id)
	assert.Equal(t, "192.168.88.1/24", addr.Address)
	assert.Equal(t, "bridge", addr.Interface)
}

func TestReadName(t *testing.T) {
	c, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/rest/interface/bridge/br-main", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{".id":"*1","name":"br-main"}`))
	})

	var br testNamedResource
	err := ReadName(context.Background(), c, &br, "br-main")
	require.NoError(t, err)
	assert.Equal(t, "*1", br.Id)
	assert.Equal(t, "br-main", br.Name())
}

func TestRequestHelpersAndRouterConfig(t *testing.T) {
	var cfg RouterConfig
	reqs := cfg.Requests()
	// System(3) + Network(5) + IP(4) + Firewall(2) + Routing(2) = 16 requests
	assert.Len(t, reqs, 16)

	r1 := requestSingle(&cfg.System.Identity)
	r2 := requestId(&BridgeConfig{}, "*1")
	r3 := requestSlice(&cfg.Network.Bridges)

	single := requests(r1)
	assert.Len(t, single, 1)

	combined := concat(r1, r2, r3)
	assert.Len(t, combined, 3)

	all := concatAll(single, combined)
	assert.Len(t, all, 4)
}
