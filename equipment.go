package zone

import (
	"errors"
	"fmt"
	"github.com/miekg/dns"
	"net"
	"regexp"
	"sort"
	"strings"
)

const (
	DEF_PORT = "53"
)

type Equipment struct {
	Zone   string
	Server string
	Port   string
}

func (e *Equipment) transfer() ([]dns.RR, error) {
	m := new(dns.Msg)
	m.SetAxfr(e.Zone)

	port := e.Port
	if port == "" {
		port = DEF_PORT
	}

	s, err := net.LookupHost(e.Server)
	if err != nil {
		return nil, err
	}

	tr := new(dns.Transfer)
	a, err := tr.In(m, net.JoinHostPort(s[0], port))
	if err != nil {
		return nil, err
	}

	var res []dns.RR
	for ex := range a {
		if ex.Error != nil {
			return nil, ex.Error
		}
		res = append(res, ex.RR...)
	}

	return res, nil
}

func (e *Equipment) lookup(name string, record uint16) ([]dns.RR, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), record)
	m.RecursionDesired = true

	port := e.Port
	if port == "" {
		port = DEF_PORT
	}

	c := new(dns.Client)
	r, _, err := c.Exchange(m, net.JoinHostPort(e.Server, port))
	if err != nil {
		return nil, err
	}
	if r.Rcode != dns.RcodeSuccess {
		return nil, errors.New(fmt.Sprintf("invalid lookup answer for %s", name))
	}

	return r.Answer, nil
}

func (e *Equipment) decode(records []dns.RR) (*Device, error) {
	d := Device{}

	for _, r := range records {
		d.Name = r.Header().Name
		switch x := r.(type) {
		case *dns.A:
			d.IP = x.A
		case *dns.CNAME:
		case *dns.TXT:
			d.Place = strings.Join(x.Txt, " ")
		case *dns.HINFO:
			d.Code = x.Os
			d.Model = x.Cpu
		case *dns.LOC:
			d.SetLocation(x.Latitude, x.Longitude, x.Altitude)
		}
	}

	return &d, nil
}

func (e *Equipment) gather(name string) (*Device, error) {
	var res []dns.RR

	// search for an A record
	ans, err := e.lookup(name, dns.TypeA)
	if err != nil {
		return nil, err
	}
	// we need at least one
	if !(len(ans) > 0) {
		return nil, nil
	}
	res = append(res, ans...)

	// gather other records ...
	txt, err := e.lookup(name, dns.TypeTXT)
	if err == nil {
		res = append(res, txt...)
	}
	hinfo, err := e.lookup(name, dns.TypeHINFO)
	if err == nil {
		res = append(res, hinfo...)
	}
	loc, err := e.lookup(name, dns.TypeLOC)
	if err == nil {
		res = append(res, loc...)
	}

	return e.decode(res)
}

func (e *Equipment) List() ([]Device, error) {

	rr, err := e.transfer()
	if err != nil {
		return nil, err
	}

	devices := make(map[string]Device)

	// only collect A record details ...
	for _, r := range rr {
		switch x := r.(type) {
		case *dns.A:
			devices[r.Header().Name] = Device{Name: r.Header().Name, IP: x.A}
		}
	}

	// gather other device details ...
	for _, r := range rr {
		d, ok := devices[r.Header().Name]
		if !ok {
			continue
		}
		switch x := r.(type) {
		case *dns.A:
		case *dns.CNAME:
		case *dns.TXT:
			d.Place = strings.Join(x.Txt, " ")
		case *dns.HINFO:
			d.Code = x.Os
			d.Model = x.Cpu
		case *dns.LOC:
			d.SetLocation(x.Latitude, x.Longitude, x.Altitude)
		}
		devices[r.Header().Name] = d
	}

	// sort by device name
	var keys []string
	for k := range devices {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// build resulting sorted array
	res := make([]Device, 0, len(devices))
	for _, k := range keys {
		res = append(res, devices[k])
	}

	return res, nil
}

func (e *Equipment) Find(name string) (*Device, error) {
	return e.gather(name)
}

func (e *Equipment) FindByIP(ip net.IP) (*Device, error) {
	s, err := net.LookupAddr(ip.String())
	if err != nil {
		return nil, err
	}
	if !(len(s) > 0) {
		return nil, nil
	}
	return e.gather(s[0])
}

func (e *Equipment) ListByModelAndCode(model, code string) ([]Device, error) {
	devices, err := e.List()
	if err != nil {
		return nil, err
	}

	res := make([]Device, 0, len(devices))
	for _, d := range devices {
		if d.Model != model {
			continue
		}
		if d.Code != code {
			continue
		}
		res = append(res, d)
	}

	return res, nil
}

func (e *Equipment) ListByModel(model string) ([]Device, error) {

	devices, err := e.List()
	if err != nil {
		return nil, err
	}

	res := make([]Device, 0, len(devices))
	for _, d := range devices {
		if d.Model != model {
			continue
		}

		res = append(res, d)
	}

	return res, nil
}

func (e *Equipment) ListByCode(code string) ([]Device, error) {
	devices, err := e.List()
	if err != nil {
		return nil, err
	}
	res := make([]Device, 0, len(devices))
	for _, d := range devices {
		if d.Code != code {
			continue
		}

		res = append(res, d)
	}

	return res, nil
}

func (e *Equipment) ListByPlace(place string) ([]Device, error) {
	devices, err := e.List()
	if err != nil {
		return nil, err
	}

	res := make([]Device, 0, len(devices))
	for _, d := range devices {
		if d.Place != place {
			continue
		}

		res = append(res, d)
	}

	return res, nil
}

func (e *Equipment) ListByNetwork(network net.IPNet) ([]Device, error) {
	devices, err := e.List()
	if err != nil {
		return nil, err
	}

	res := make([]Device, 0, len(devices))
	for _, d := range devices {
		if !network.Contains(d.IP) {
			continue
		}

		res = append(res, d)
	}

	return res, nil
}

func (e *Equipment) MatchByModel(model string) ([]Device, error) {

	re, err := regexp.Compile(model)
	if err != nil {
		return nil, err
	}

	devices, err := e.List()
	if err != nil {
		return nil, err
	}
	res := make([]Device, 0, len(devices))
	for _, d := range devices {
		if !re.MatchString(d.Model) {
			continue
		}

		res = append(res, d)
	}

	return res, nil
}

func (e *Equipment) MatchByModelAndCode(model, code string) ([]Device, error) {

	re, err := regexp.Compile(model)
	if err != nil {
		return nil, err
	}

	devices, err := e.List()
	if err != nil {
		return nil, err
	}
	res := make([]Device, 0, len(devices))
	for _, d := range devices {
		if !re.MatchString(d.Model) {
			continue
		}
		if d.Code != code {
			continue
		}

		res = append(res, d)
	}

	return res, nil
}
