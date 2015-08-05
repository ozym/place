package zone

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"regexp"
)

type Devices struct {
	List []*Device
}

func LoadLocal(server string, zones, reverse []string) (*Devices, error) {
	s := Service{
		Server: server,
		Port:   "53",
	}

	l, err := s.List(zones, reverse)
	if err != nil {
		return nil, err
	}

	d := Devices{List: l}

	return &d, nil
}

func LoadRemote(server string) (*Devices, error) {

	s := Service{
		Server: server,
		Port:   "9001",
	}

	host, err := s.ServerPort()
	if err != nil {
		return nil, err
	}

	abs := url.URL{Scheme: "http", Host: host}

	u, err := url.Parse(server)
	if err != nil {
		return nil, err
	}

	var l []*Device
	res, err := http.Get(abs.ResolveReference(u).String())
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(body, &l)
	if err != nil {
		return nil, err
	}

	d := Devices{List: l}

	return &d, nil
}

func (d *Devices) Find(name string) *Device {
	for _, s := range d.List {
		if s.Name == name {
			return s
		}
	}
	return nil
}

func (d *Devices) FindByIP(ip net.IP) *Device {
	for _, s := range d.List {
		if s.IP.Equal(ip) {
			return s
		}
	}
	return nil
}

func (d *Devices) ListByModel(model string) []*Device {
	var l []*Device

	for _, s := range d.List {
		if !s.HasModel(model) {
			continue
		}
		l = append(l, s)
	}

	return l
}

func (d *Devices) ListByCode(code string) []*Device {
	var l []*Device

	for _, s := range d.List {
		if !s.HasCode(code) {
			continue
		}
		l = append(l, s)
	}

	return l
}

func (d *Devices) ListByPlace(place string) []*Device {
	var l []*Device

	for _, s := range d.List {
		if !s.AtPlace(place) {
			continue
		}
		l = append(l, s)
	}

	return l
}

func (d *Devices) ListByModelAndCode(model, code string) []*Device {

	var l []*Device

	for _, s := range d.List {
		if !s.HasModel(model) {
			continue
		}
		if !s.HasCode(code) {
			continue
		}
		l = append(l, s)
	}

	return l
}

func (d *Devices) ListByNetwork(network net.IPNet) []*Device {
	var l []*Device

	for _, s := range d.List {
		if !s.InNetwork(network) {
			continue
		}
		l = append(l, s)
	}

	return l
}

func (d *Devices) MatchByName(name string) ([]*Device, error) {
	var l []*Device

	re, err := regexp.Compile(name)
	if err != nil {
		return nil, err
	}

	for _, s := range d.List {
		if !re.MatchString(s.Name) {
			continue
		}
		l = append(l, s)
	}

	return l, nil
}

func (d *Devices) MatchByModel(model string) ([]*Device, error) {
	var l []*Device

	re, err := regexp.Compile(model)
	if err != nil {
		return nil, err
	}

	for _, s := range d.List {
		if !re.MatchString(s.Model) {
			continue
		}
		l = append(l, s)
	}

	return l, nil
}

func (d *Devices) MatchByPlace(place string) ([]*Device, error) {
	var l []*Device

	re, err := regexp.Compile(place)
	if err != nil {
		return nil, err
	}

	for _, s := range d.List {
		if !re.MatchString(s.Place) {
			continue
		}
		l = append(l, s)
	}

	return l, nil
}
