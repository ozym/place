package zone

import (
	"fmt"
	"net"
)

const (
	LOC_EQUATOR       = 1 << 31 // RFC 1876, Section 2.
	LOC_PRIMEMERIDIAN = 1 << 31 // RFC 1876, Section 2.
	LOC_DEGREES       = 60.0 * 60.0 * 1000.0
	LOC_ALTITUDEBASE  = 100000.0
)

type Device struct {
	Name      string
	IP        net.IP
	Place     string
	Code      string
	Model     string
	Latitude  float64
	Longitude float64
	Height    float64
}

func (e *Device) SetLocation(lat, lon, alt uint32) {
	if lat > LOC_EQUATOR {
		e.Latitude = (float64)(lat-LOC_EQUATOR) / LOC_DEGREES
	} else {
		e.Latitude = (float64)(LOC_EQUATOR-lat) / -LOC_DEGREES
	}

	if lon > LOC_PRIMEMERIDIAN {
		e.Longitude = (float64)(lon-LOC_PRIMEMERIDIAN) / LOC_DEGREES
	} else {
		e.Longitude = (float64)(LOC_PRIMEMERIDIAN-lon) / -LOC_DEGREES
	}

	e.Height = float64(alt)/100.0 - LOC_ALTITUDEBASE
}

func (e *Device) String() string {
	return fmt.Sprintf("Host: %s, IP: %s, Place: \"%s\", Model: \"%s\", Code: %s, Latitude: %g, Longitude: %g, Height: %g",
                e.Name, e.IP, e.Place, e.Code, e.Model, e.Latitude, e.Longitude, e.Height)
}
