package zone

import (
	"errors"
	"fmt"
	"github.com/miekg/dns"
	"net"
	"sort"
	"strings"
)

type Service struct {
	Zone   string
	Server string
}

func ServerPort(server, port string) (string, error) {

	s := server
	p := port

	tmp := strings.Split(server, ":")
	if len(tmp) > 1 {
		s, p = tmp[0], tmp[1]
	}

	h, err := net.LookupHost(s)
	if err != nil {
		return "", err
	}

	return net.JoinHostPort(h[0], p), nil
}

func (s *Service) Transfer() ([]dns.RR, error) {
	m := new(dns.Msg)
	m.SetAxfr(s.Zone)

	h, err := ServerPort(s.Server, "53")
	if err != nil {
		return nil, err
	}

	tr := new(dns.Transfer)
	a, err := tr.In(m, h)
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

func (s *Service) Lookup(name string, record uint16) ([]dns.RR, error) {
	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(name), record)
	m.RecursionDesired = true

	h, err := ServerPort(s.Server, "53")
	if err != nil {
		return nil, err
	}

	c := new(dns.Client)
	r, _, err := c.Exchange(m, h)
	if err != nil {
		return nil, err
	}

	if r.Rcode != dns.RcodeSuccess {
		return nil, errors.New(fmt.Sprintf("invalid lookup answer for %s", name))
	}

	return r.Answer, nil
}

func (s *Service) Decode(records []dns.RR) *Device {
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

	return &d
}

func (s *Service) Find(name string) (*Device, error) {
	var res []dns.RR

	// search for an A record
	ans, err := s.Lookup(name, dns.TypeA)
	if err != nil {
		return nil, err
	}
	// we need at least one
	if !(len(ans) > 0) {
		return nil, nil
	}
	res = append(res, ans...)

	// gather other records ...
	txt, err := s.Lookup(name, dns.TypeTXT)
	if err == nil {
		res = append(res, txt...)
	}
	hinfo, err := s.Lookup(name, dns.TypeHINFO)
	if err == nil {
		res = append(res, hinfo...)
	}
	loc, err := s.Lookup(name, dns.TypeLOC)
	if err == nil {
		res = append(res, loc...)
	}

	return s.Decode(res), nil
}

func (s *Service) FindByIP(ip net.IP) (*Device, error) {
	h, err := net.LookupAddr(ip.String())
	if err != nil {
		return nil, err
	}
	if !(len(h) > 0) {
		return nil, nil
	}
	return s.Find(h[0])
}

func (s *Service) List() ([]*Device, error) {

	rr, err := s.Transfer()
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
	res := make([]*Device, 0, len(devices))
	for _, k := range keys {
		d := devices[k]
		res = append(res, &d)
	}

	return res, nil
}
