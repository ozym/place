package zone

import (
	"fmt"
	"github.com/miekg/dns"
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

func (d *Device) SetLocation(lat, lon, alt uint32) {
	if lat > LOC_EQUATOR {
		d.Latitude = (float64)(lat-LOC_EQUATOR) / LOC_DEGREES
	} else {
		d.Latitude = (float64)(LOC_EQUATOR-lat) / -LOC_DEGREES
	}

	if lon > LOC_PRIMEMERIDIAN {
		d.Longitude = (float64)(lon-LOC_PRIMEMERIDIAN) / LOC_DEGREES
	} else {
		d.Longitude = (float64)(LOC_PRIMEMERIDIAN-lon) / -LOC_DEGREES
	}

	d.Height = float64(alt)/100.0 - LOC_ALTITUDEBASE
}

func (d *Device) Hostname() string {
	l := dns.SplitDomainName(d.Name)
	if len(l) > 0 {
		return l[0]
	}
	return ""
}

func (d *Device) String() string {
	return fmt.Sprintf("Host: %s, IP: %s, Place: \"%s\", Model: \"%s\", Code: %s, Latitude: %g, Longitude: %g, Height: %g",
		d.Name, d.IP, d.Place, d.Model, d.Code, d.Latitude, d.Longitude, d.Height)
}
