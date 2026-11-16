# adsbdb-client

[![Lint](https://github.com/aviation-tragic-au/adsbdb-client/actions/workflows/lint.yml/badge.svg)](https://github.com/aviation-tragic-au/adsbdb-client/actions/workflows/lint.yml) [![Test](https://github.com/aviation-tragic-au/adsbdb-client/actions/workflows/test.yml/badge.svg)](https://github.com/aviation-tragic-au/adsbdb-client/actions/workflows/test.yml)

A Go client for [ADSBDB](https://www.adsbdb.com), a free, keyless database
of aircraft (by Mode S hex or registration), routes (by callsign) and
airlines. Its answers change rarely, so cache them, and send a User-Agent
that identifies your program.

## Install

```sh
go get github.com/aviation-tragic-au/adsbdb-client
```

## Usage

```go
package main

import (
    "context"
    "errors"
    "fmt"
    "log"

    adsbdb "github.com/aviation-tragic-au/adsbdb-client"
)

func main() {
    client, err := adsbdb.NewClient(nil, adsbdb.Config{
        UserAgent: "my-program/1.0 (+https://example.com)",
    })
    if err != nil {
        log.Fatal(err)
    }

    ctx := context.Background()

    route, err := client.GetRoute(ctx, "QFA615")
    switch {
    case errors.Is(err, adsbdb.ErrNotFound):
        fmt.Println("no route for QFA615")
    case err != nil:
        log.Fatal(err)
    default:
        fmt.Printf("%s -> %s\n", route.Origin.IATACode, route.Destination.IATACode)
    }

    // The aircraft, and its route under a callsign, in one request
    aircraft, route, err := client.GetAircraftWithRoute(ctx, "VH-VZS", "QFA615")
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(aircraft.ICAOType, aircraft.RegisteredOwner, route != nil)
}
```

### Methods

Every method takes a `context.Context` first.

| Method | Returns |
|---|---|
| `GetRoute(ctx, callsign)` | The route flown under a callsign |
| `GetAircraft(ctx, hexOrRegistration)` | An aircraft by Mode S hex or registration |
| `GetAircraftWithRoute(ctx, hexOrRegistration, callsign)` | Both, in one request (the route is nil when unknown) |
| `GetAirlines(ctx, code)` | Airlines with an ICAO or IATA code (an IATA code can be shared) |
| `GetAirline(ctx, icao)` | The airline with an ICAO code |
| `GetModeS(ctx, nNumber)` / `GetNNumber(ctx, hex)` | Convert between a US N-number and its Mode S hex |

ADSBDB not knowing something is `ErrNotFound`, a definitive answer worth
caching. A 429 matches `ErrRateLimited` (an `*adsbv2.RateLimitError` with
the `Retry-After` delay); any other non-200, including the 400 ADSBDB sends
for a code it can't parse, is an `*adsbv2.StatusError`.

### Configuration

| Field        | Description |
|--------------|-------------|
| `UserAgent`  | Identifies your program (default `adsbv2.DefaultUserAgent`) |
| `URL`        | API base (default `https://api.adsbdb.com`) |
| `HTTPClient` | Your own `*http.Client` (the default times out after 10 seconds) |

`Config` carries `json` and `mapstructure` tags, so it can be decoded
straight from a config file or Viper.

## License

Copyright 2026 Dean Stalker

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
