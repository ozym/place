package zone

import (
	"encoding/json"
	"github.com/miekg/dns"
	"net"
	"strings"
)

const (
	LOC_EQUATOR       = 1 << 31 // RFC 1876, Section 2.
	LOC_PRIMEMERIDIAN = 1 << 31 // RFC 1876, Section 2.
	LOC_DEGREES       = 60.0 * 60.0 * 1000.0
	LOC_ALTITUDEBASE  = 100000.0
)

type Device struct {
	Name      string   `json:"name"`
	IP        net.IP   `json:"ip"`
	Reverse   []net.IP `json:"reverse"`
	Aliases   []string `json:"aliases"`
	Place     string   `json:"place"`
	Model     string   `json:"model"`
	Code      string   `json:"code"`
	Latitude  float64  `json:"latitude"`
	Longitude float64  `json:"longitude"`
	Height    float64  `json:"height"`
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

func (d *Device) Location() (uint32, uint32, uint32) {

	lat := uint32((d.Latitude * LOC_DEGREES) + LOC_EQUATOR)
	lon := uint32((d.Longitude * LOC_DEGREES) + LOC_PRIMEMERIDIAN)
	alt := uint32((d.Height + LOC_ALTITUDEBASE) * 100.0)

	return lat, lon, alt
}

func (d *Device) Hostname() string {
	l := dns.SplitDomainName(d.Name)
	if len(l) > 0 {
		return l[0]
	}
	return d.Name
}

func (d *Device) HasName(name string) bool {
	return strings.EqualFold(d.Name, name)
}
func (d *Device) HasAddress(ip net.IP) bool {
	if d.IP.Equal(ip) {
		return true
	}
	for _, a := range d.Reverse {
		if !a.Equal(ip) {
			continue
		}
		return true
	}
	return false
}
func (d *Device) InNetwork(network net.IPNet) bool {
	if network.Contains(d.IP) {
		return true
	}
	for _, a := range d.Reverse {
		if !network.Contains(a) {
			continue
		}
		return true
	}
	return false
}
func (d *Device) AtPlace(place string) bool {
	return strings.EqualFold(d.Place, place)
}
func (d *Device) AtLocation(lat, lon, alt uint32) bool {
	la, lo, a := d.Location()
	if la != lat {
		return false
	}
	if lo != lon {
		return false
	}
	if a != alt {
		return false
	}
	return true
}
func (d *Device) HasCode(code string) bool {
	return strings.EqualFold(d.Code, code)
}
func (d *Device) HasModel(model string) bool {
	return d.Model == model
}

func (d *Device) Equal(device *Device) bool {
	if !d.HasName(device.Name) {
		return false
	}
	if !d.HasAddress(device.IP) {
		return false
	}
	if !d.HasCode(device.Code) {
		return false
	}
	if !d.HasModel(device.Model) {
		return false
	}
	if !d.AtLocation(device.Location()) {
		return false
	}
	return true
}

func (d *Device) String() string {
	m, err := json.Marshal(d)
	if err != nil {
		return ""
	}
	return (string)(m)
}
