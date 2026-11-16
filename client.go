// Package adsbdb is a client for the ADSBDB API (https://www.adsbdb.com):
// a free, keyless database of aircraft by Mode S hex or registration,
// routes by callsign, and airlines. Its answers change rarely, so callers
// should cache them.
package adsbdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aviation-tragic-au/adsbv2"
	"go.uber.org/zap"
)

const (
	// DefaultURL is ADSBDB's API base, used when Config.URL is empty.
	DefaultURL = "https://api.adsbdb.com"

	// DefaultTimeout bounds a request when Config.HTTPClient is unset.
	DefaultTimeout = 10 * time.Second
)

var (
	// ErrNotFound is returned when ADSBDB doesn't know the aircraft,
	// callsign or airline: a definitive answer worth caching, unlike a
	// transport error.
	ErrNotFound = errors.New("adsbdb: not found")

	// ErrRateLimited matches, under errors.Is, a 429. The error is an
	// *adsbv2.RateLimitError carrying the Retry-After delay.
	ErrRateLimited = adsbv2.ErrRateLimited
)

// Config configures a Client. Everything is optional. The mapstructure
// tags let a host decode it straight from viper or similar.
type Config struct {
	// URL is the API base. Empty means DefaultURL.
	URL string `json:"url" mapstructure:"url"`

	// UserAgent identifies the program to ADSBDB. Empty means
	// adsbv2.DefaultUserAgent.
	UserAgent string `json:"userAgent" mapstructure:"userAgent"`

	// HTTPClient sends the requests. Nil means a client with
	// DefaultTimeout.
	HTTPClient *http.Client `json:"-" mapstructure:"-"`
}

// Client is a client for the ADSBDB API. It is safe for concurrent use.
type Client struct {
	logger     *zap.Logger
	httpClient *http.Client
	url        string
	userAgent  string
}

// NewClient creates an ADSBDB client. A nil logger logs nothing.
func NewClient(logger *zap.Logger, cfg Config) (*Client, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	base := strings.TrimRight(cfg.URL, "/")
	if base == "" {
		base = DefaultURL
	}
	if _, err := url.Parse(base); err != nil {
		return nil, fmt.Errorf("adsbdb: invalid URL: %w", err)
	}

	userAgent := cfg.UserAgent
	if userAgent == "" {
		userAgent = adsbv2.DefaultUserAgent
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultTimeout}
	}

	return &Client{
		logger:     logger.Named("adsbdb"),
		httpClient: httpClient,
		url:        base,
		userAgent:  userAgent,
	}, nil
}

// GetRoute returns the route flown under a callsign ("QFA615"), or
// ErrNotFound.
func (c *Client) GetRoute(ctx context.Context, callsign string) (*FlightRoute, error) {
	callsign = strings.ToUpper(strings.TrimSpace(callsign))
	if callsign == "" {
		return nil, errors.New("adsbdb: a callsign is required")
	}

	var out struct {
		Response struct {
			FlightRoute *FlightRoute `json:"flightroute"`
		} `json:"response"`
	}
	if err := c.get(ctx, "/v0/callsign/"+url.PathEscape(callsign), nil, &out); err != nil {
		return nil, err
	}
	if out.Response.FlightRoute == nil {
		return nil, ErrNotFound
	}
	return out.Response.FlightRoute, nil
}

// GetAircraft returns the aircraft with a Mode S hex ("7C6DE6") or a
// registration ("VH-VZS"), or ErrNotFound.
func (c *Client) GetAircraft(ctx context.Context, hexOrRegistration string) (*Aircraft, error) {
	aircraft, _, err := c.aircraft(ctx, hexOrRegistration, "")
	return aircraft, err
}

// GetAircraftWithRoute returns the aircraft, as GetAircraft does, and the
// route flown under callsign in the same request. The route is nil when
// ADSBDB doesn't know the callsign; ErrNotFound means it doesn't know the
// aircraft.
func (c *Client) GetAircraftWithRoute(ctx context.Context, hexOrRegistration, callsign string) (*Aircraft, *FlightRoute, error) {
	callsign = strings.ToUpper(strings.TrimSpace(callsign))
	if callsign == "" {
		return nil, nil, errors.New("adsbdb: a callsign is required")
	}
	return c.aircraft(ctx, hexOrRegistration, callsign)
}

func (c *Client) aircraft(ctx context.Context, hexOrRegistration, callsign string) (*Aircraft, *FlightRoute, error) {
	code := strings.ToUpper(strings.TrimSpace(hexOrRegistration))
	if code == "" {
		return nil, nil, errors.New("adsbdb: a hex or registration is required")
	}

	var query url.Values
	if callsign != "" {
		query = url.Values{"callsign": {callsign}}
	}

	var out struct {
		Response struct {
			Aircraft    *Aircraft    `json:"aircraft"`
			FlightRoute *FlightRoute `json:"flightroute"`
		} `json:"response"`
	}
	if err := c.get(ctx, "/v0/aircraft/"+url.PathEscape(code), query, &out); err != nil {
		return nil, nil, err
	}
	if out.Response.Aircraft == nil {
		return nil, nil, ErrNotFound
	}
	return out.Response.Aircraft, out.Response.FlightRoute, nil
}

// GetAirlines returns the airlines with an ICAO ("QFA") or IATA ("QF")
// code. An IATA code can belong to several airlines.
func (c *Client) GetAirlines(ctx context.Context, code string) ([]*Airline, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return nil, errors.New("adsbdb: an airline code is required")
	}

	var out struct {
		Response []*Airline `json:"response"`
	}
	if err := c.get(ctx, "/v0/airline/"+url.PathEscape(code), nil, &out); err != nil {
		return nil, err
	}
	if len(out.Response) == 0 {
		return nil, ErrNotFound
	}
	return out.Response, nil
}

// GetAirline returns the airline with an ICAO code (the three letters a
// callsign starts with), or ErrNotFound. Useful when there's no route for
// the flight itself.
func (c *Client) GetAirline(ctx context.Context, icao string) (*Airline, error) {
	airlines, err := c.GetAirlines(ctx, icao)
	if err != nil {
		return nil, err
	}
	for _, a := range airlines {
		if a != nil && strings.EqualFold(a.ICAO, strings.TrimSpace(icao)) {
			return a, nil
		}
	}
	return nil, ErrNotFound
}

// GetModeS converts a US N-number ("N12345") to its Mode S hex.
func (c *Client) GetModeS(ctx context.Context, nNumber string) (string, error) {
	return c.convert(ctx, "n-number", nNumber)
}

// GetNNumber converts a US Mode S hex to its N-number.
func (c *Client) GetNNumber(ctx context.Context, hex string) (string, error) {
	return c.convert(ctx, "mode-s", hex)
}

func (c *Client) convert(ctx context.Context, kind, code string) (string, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return "", fmt.Errorf("adsbdb: a %s is required", kind)
	}

	var out struct {
		Response string `json:"response"`
	}
	if err := c.get(ctx, "/v0/"+kind+"/"+url.PathEscape(code), nil, &out); err != nil {
		return "", err
	}
	if out.Response == "" {
		return "", ErrNotFound
	}
	return out.Response, nil
}

// get sends a GET and decodes a 200's body into out. A 404 is ErrNotFound
// and a 429 a rate limit error. ADSBDB answers a code it can't parse with
// a 400; that's an error too, not ErrNotFound.
func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url+path, http.NoBody)
	if err != nil {
		return fmt.Errorf("adsbdb: unable to build request: %w", err)
	}
	if len(query) > 0 {
		req.URL.RawQuery = query.Encode()
	}
	req.Header.Set("User-Agent", c.userAgent)

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("adsbdb: request failed: %w", err)
	}
	defer func() { _ = res.Body.Close() }()

	switch res.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return ErrNotFound
	default:
		// adsbv2 maps a 429 to its rate limit error, and anything else to
		// a StatusError carrying the start of the body.
		err := adsbv2.CheckStatus(res)
		c.logger.Debug("request failed", zap.String("path", path), zap.Error(err))
		return fmt.Errorf("adsbdb: %w", err)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("adsbdb: unable to read response: %w", err)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("adsbdb: unable to decode response: %w", err)
	}
	return nil
}
