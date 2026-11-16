package adsbdb

// FlightRoute is ADSBDB's route for a callsign.
type FlightRoute struct {
	Callsign     string   `json:"callsign"`
	CallsignICAO string   `json:"callsign_icao"`
	CallsignIATA string   `json:"callsign_iata"`
	Airline      *Airline `json:"airline"`
	Origin       *Airport `json:"origin"`
	Destination  *Airport `json:"destination"`
}

// Airline is an operator, on a FlightRoute or from an airline lookup.
type Airline struct {
	Name       string `json:"name"`
	ICAO       string `json:"icao"`
	IATA       string `json:"iata"`
	Country    string `json:"country"`
	CountryISO string `json:"country_iso"`
	// Callsign is the airline's radio callsign ("QANTAS").
	Callsign string `json:"callsign"`
}

// Airport is a route endpoint.
type Airport struct {
	ICAOCode     string  `json:"icao_code"`
	IATACode     string  `json:"iata_code"`
	Name         string  `json:"name"`
	Municipality string  `json:"municipality"`
	CountryISO   string  `json:"country_iso_name"`
	CountryName  string  `json:"country_name"`
	Latitude     float64 `json:"latitude"`
	Longitude    float64 `json:"longitude"`
	// Elevation is in feet.
	Elevation int `json:"elevation"`
}

// Aircraft is ADSBDB's registration record for an aircraft.
type Aircraft struct {
	// Type is the full type name ("737NG 838/W").
	Type string `json:"type"`
	// ICAOType is the ICAO type designator ("B738").
	ICAOType     string `json:"icao_type"`
	Manufacturer string `json:"manufacturer"`
	// ModeS is the 24-bit ICAO address in hex, upper case.
	ModeS           string `json:"mode_s"`
	Registration    string `json:"registration"`
	RegisteredOwner string `json:"registered_owner"`
	OwnerCountry    string `json:"registered_owner_country_name"`
	OwnerCountryISO string `json:"registered_owner_country_iso_name"`
	// OperatorFlagCode is the owner's ICAO airline code, when it has one.
	OperatorFlagCode  string `json:"registered_owner_operator_flag_code"`
	PhotoURL          string `json:"url_photo"`
	PhotoThumbnailURL string `json:"url_photo_thumbnail"`
}
