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

// Device describes the DNS stored equipment information.
// It is assumed that each piece of equipment has a single A record pointing
// to a definitive DNS Name and Address. Possible equipment Aliases
// can also be stored via CNAME lookups, likewise Mapping IP addresses can be
// stored via PTR records pointing to CNAME entries. Also stored are Place,
// Model details, instrument or site Codes and Place location information.
type Device struct {
	Name      string            `json:"name"`      // full dns name
	IP        net.IP            `json:"ip"`        // primary ip address (A)
	Reverse   []net.IP          `json:"reverse"`   // primary lookups (PTR)
	Mapping   map[string]net.IP `json:"mapping"`   // secondary lookups (PTR/CNAME)
	Aliases   []string          `json:"aliases"`   // other names (CNAME)
	Place     string            `json:"place"`     // full place name (TXT)
	Model     string            `json:"model"`     // equipment model (HINFO)
	Code      string            `json:"code"`      // equipment site code (HINFO)
	Latitude  float64           `json:"latitude"`  // place latitude (LOC)
	Longitude float64           `json:"longitude"` // place longitude (LOC)
	Height    float64           `json:"height"`    // place height (LOC)
}

func CopyIP(ip net.IP) net.IP {
	p := make(net.IP, len(ip))
	copy(p, ip)
	return p
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

func (d *Device) HasAlias(alias string) bool {
	for _, a := range d.Aliases {
		if a != alias {
			continue
		}
		return true
	}
	return false
}

func (d *Device) HasReverse(ip net.IP) bool {
	for _, a := range d.Reverse {
		if !a.Equal(ip) {
			continue
		}
		return true
	}
	return false
}

func (d *Device) HasIP(ip net.IP) bool {
	for _, a := range d.Mapping {
		if !a.Equal(ip) {
			continue
		}
		return true
	}
	return false
}

func (d *Device) HasMapping(name string, ip net.IP) bool {
	for n, i := range d.Mapping {
		if n == name && i.Equal(ip) {
			return true
		}
	}
	return false
}

func (d *Device) String() string {
	m, err := json.Marshal(d)
	if err != nil {
		return ""
	}
	return (string)(m)
}
