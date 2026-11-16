package adsbdb

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/aviation-tragic-au/adsbv2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient serves canned ADSBDB answers, shaped like the live API's.
func newTestClient(t *testing.T) *Client {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "test-agent/1.0", r.Header.Get("User-Agent"))
		switch r.URL.EscapedPath() {
		case "/v0/callsign/QFA615":
			_, _ = io.WriteString(w, `{"response":{"flightroute":{"callsign":"QFA615","callsign_iata":"QF615",
				"airline":{"name":"Qantas","icao":"QFA","iata":"QF"},
				"origin":{"icao_code":"YBBN","iata_code":"BNE","elevation":13},
				"destination":{"icao_code":"YMML","iata_code":"MEL","elevation":434}}}}`)
		case "/v0/aircraft/7C6DE6", "/v0/aircraft/VH-VZS":
			route := ""
			if r.URL.Query().Get("callsign") == "QFA615" {
				route = `,"flightroute":{"callsign":"QFA615","origin":{"icao_code":"YBBN"},"destination":{"icao_code":"YMML"}}`
			}
			_, _ = io.WriteString(w, `{"response":{"aircraft":{"icao_type":"B738","mode_s":"7C6DE6","registration":"VH-VZS",
				"registered_owner":"Qantas","registered_owner_country_iso_name":"AU"}`+route+`}}`)
		case "/v0/airline/QF":
			_, _ = io.WriteString(w, `{"response":[{"name":"Eastern Australia Airlines","icao":"EAQ","iata":"QF"},{"name":"Qantas","icao":"QFA","iata":"QF"}]}`)
		case "/v0/airline/QFA":
			_, _ = io.WriteString(w, `{"response":[{"name":"Qantas","icao":"QFA","iata":"QF"}]}`)
		case "/v0/airline/EMP":
			_, _ = io.WriteString(w, `{"response":[]}`)
		case "/v0/n-number/N12345":
			_, _ = io.WriteString(w, `{"response":"A061D9"}`)
		case "/v0/mode-s/A061D9":
			_, _ = io.WriteString(w, `{"response":"N12345"}`)
		case "/v0/aircraft/BAD%2FCODE":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"response":"invalid aircraft"}`)
		case "/v0/callsign/SLOW1":
			w.Header().Set("Retry-After", "5")
			w.WriteHeader(http.StatusTooManyRequests)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"response":"unknown callsign"}`)
		}
	}))
	t.Cleanup(srv.Close)

	c, err := NewClient(nil, Config{URL: srv.URL, UserAgent: "test-agent/1.0"})
	require.NoError(t, err)
	return c
}

func TestNewClient(t *testing.T) {
	c, err := NewClient(nil, Config{})
	require.NoError(t, err)
	assert.Equal(t, DefaultURL, c.url)
	assert.Equal(t, adsbv2.DefaultUserAgent, c.userAgent)
	assert.Equal(t, DefaultTimeout, c.httpClient.Timeout)

	_, err = NewClient(nil, Config{URL: "http://bad\x7f"})
	require.Error(t, err)
}

func TestGetRoute(t *testing.T) {
	c := newTestClient(t)

	route, err := c.GetRoute(context.Background(), " qfa615 ")
	require.NoError(t, err)
	assert.Equal(t, "BNE", route.Origin.IATACode)
	assert.Equal(t, "MEL", route.Destination.IATACode)
	assert.Equal(t, 434, route.Destination.Elevation)
	assert.Equal(t, "Qantas", route.Airline.Name)

	_, err = c.GetRoute(context.Background(), "ZZZ9999")
	require.ErrorIs(t, err, ErrNotFound)

	_, err = c.GetRoute(context.Background(), "")
	require.ErrorContains(t, err, "callsign is required")
}

func TestGetAircraft(t *testing.T) {
	c := newTestClient(t)

	for _, code := range []string{"7c6de6", "VH-VZS"} {
		ac, err := c.GetAircraft(context.Background(), code)
		require.NoError(t, err, code)
		assert.Equal(t, "VH-VZS", ac.Registration)
		assert.Equal(t, "AU", ac.OwnerCountryISO)
	}

	_, err := c.GetAircraft(context.Background(), "ZZZZZZ")
	require.ErrorIs(t, err, ErrNotFound)

	// a 400 for an unparseable code is an error, not "not found"
	_, err = c.GetAircraft(context.Background(), "bad/code")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrNotFound)
}

func TestGetAircraftWithRoute(t *testing.T) {
	c := newTestClient(t)

	ac, route, err := c.GetAircraftWithRoute(context.Background(), "7C6DE6", "qfa615")
	require.NoError(t, err)
	assert.Equal(t, "VH-VZS", ac.Registration)
	require.NotNil(t, route)
	assert.Equal(t, "YMML", route.Destination.ICAOCode)

	ac, route, err = c.GetAircraftWithRoute(context.Background(), "7C6DE6", "XYZ1")
	require.NoError(t, err)
	assert.NotNil(t, ac)
	assert.Nil(t, route)

	_, _, err = c.GetAircraftWithRoute(context.Background(), "7C6DE6", " ")
	require.ErrorContains(t, err, "callsign is required")
}

func TestAirlines(t *testing.T) {
	c := newTestClient(t)

	all, err := c.GetAirlines(context.Background(), "qf")
	require.NoError(t, err)
	assert.Len(t, all, 2)

	a, err := c.GetAirline(context.Background(), "QFA")
	require.NoError(t, err)
	assert.Equal(t, "Qantas", a.Name)

	// an IATA code isn't an ICAO match
	_, err = c.GetAirline(context.Background(), "QF")
	require.ErrorIs(t, err, ErrNotFound)

	_, err = c.GetAirlines(context.Background(), "EMP")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestConversions(t *testing.T) {
	c := newTestClient(t)

	hex, err := c.GetModeS(context.Background(), "n12345")
	require.NoError(t, err)
	assert.Equal(t, "A061D9", hex)

	n, err := c.GetNNumber(context.Background(), "a061d9")
	require.NoError(t, err)
	assert.Equal(t, "N12345", n)

	_, err = c.GetNNumber(context.Background(), "000000")
	require.ErrorIs(t, err, ErrNotFound)
}

func TestRateLimited(t *testing.T) {
	_, err := newTestClient(t).GetRoute(context.Background(), "SLOW1")
	require.ErrorIs(t, err, ErrRateLimited)
	var rl *adsbv2.RateLimitError
	require.ErrorAs(t, err, &rl)
	assert.Equal(t, 5*time.Second, rl.RetryAfter)
}

func TestCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	c, err := NewClient(nil, Config{URL: srv.URL})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err = c.GetRoute(ctx, "QFA615")
	require.ErrorIs(t, err, context.DeadlineExceeded)
}
