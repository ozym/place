package zone

import (
	"errors"
	"fmt"
	"github.com/miekg/dns"
	"net"
	"sort"
	"strings"
	"time"
)

type Service struct {
	Server string
	Key    string
	Secret string
	Port   string
}

func NewService(server string) *Service {
	return &Service{
		Server: server,
		Port:   "53",
	}
}

func (s *Service) ServerPort() (string, error) {

	port := s.Port
	if port == "" {
		port = "53"
	}

	sp := s.Server
	if strings.IndexByte(s.Server, ':') < 0 {
		sp = s.Server + ":" + port
	}

	n, p, err := net.SplitHostPort(sp)
	if err != nil {
		return "", err
	}

	h, err := net.LookupHost(n)
	if err != nil {
		return "", err
	}

	return net.JoinHostPort(h[0], p), nil
}

func (s *Service) Transfer(zone string) ([]dns.RR, error) {
	m := new(dns.Msg)
	m.SetAxfr(zone)

	h, err := s.ServerPort()
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

	h, err := s.ServerPort()
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
			d.IP = CopyIP(x.A)
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

func (s *Service) List(zones, reverse []string) ([]*Device, error) {
	devices := make(map[string]Device)

	// reverse lookups ....
	ptrs := make(map[string]string)
	for _, z := range reverse {
		rr, err := s.Transfer(z)
		if err != nil {
			return nil, err
		}
		// only collect PTR record details ...
		for _, r := range rr {
			switch x := r.(type) {
			case *dns.PTR:
				s := strings.Split(strings.Replace(x.Header().Name, z, "", -1)+
					strings.Replace(z, ".in-addr.arpa.", "", -1), ".")
				for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
					s[i], s[j] = s[j], s[i]
				}
				ptrs[strings.Join(s, ".")] = x.Ptr
			}
		}
	}

	// recover dns entries
	var rr []dns.RR
	for _, z := range zones {

		r, err := s.Transfer(z)
		if err != nil {
			return nil, err
		}
		rr = append(rr, r...)

	}

	// search for A and CNAME records
	cnames := make(map[string]string)
	for _, r := range rr {
		switch x := r.(type) {
		case *dns.A:
			devices[r.Header().Name] = Device{Name: r.Header().Name, IP: CopyIP(x.A)}
		case *dns.PTR:
		case *dns.CNAME:
			//cnames[x.Target] = append(cnames[x.Target], r.Header().Name)
			cnames[r.Header().Name] = x.Target //] = append(cnames[x.Target], r.Header().Name)
		case *dns.TXT:
		case *dns.HINFO:
		case *dns.LOC:
		}
	}

	// gather alias addresses ...
	for c, n := range cnames {
		if _, ok := devices[c]; ok {
			continue
		}
		if d, ok := devices[n]; ok {
			d.Aliases = append(d.Aliases, c)

			devices[n] = d
		}
	}

	// gather reverse lookups ...
	for a, n := range ptrs {
		if ip := net.ParseIP(a); ip != nil {
			if d, ok := devices[n]; ok {
				d.Reverse = append(d.Reverse, ip)

				devices[n] = d
			}
		}
	}

	// gather mappings ...
	for a, n := range ptrs {
		if ip := net.ParseIP(a); ip != nil {
			if c, ok := cnames[n]; ok {
				if d, ok := devices[c]; ok {
					if d.Mapping == nil {
						d.Mapping = make(map[string]net.IP)
					}
					d.Mapping[n] = ip

					devices[c] = d
				}
			}
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
		case *dns.PTR:
		case *dns.CNAME:
		case *dns.TXT:
			d.Place = strings.Join(x.Txt, " ")
		case *dns.HINFO:
			d.Model = x.Cpu
			d.Code = x.Os
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

// see RFC1876 - A Means for Expressing Location Information in the Domain Name System
func cm2size(cms uint32) uint8 {
	var e, v uint32

	for v = cms; v >= 10; v = v / 10 {
		e = e + 1
	}

	return (uint8(v) << 4) | uint8(e&0x0f)
}

// build an OPT RR to allow larger buffer sizes
func (d *Device) ToOPT() *dns.OPT {

	rr := &dns.OPT{
		Hdr: dns.RR_Header{Name: ".", Rrtype: dns.TypeOPT, Class: dns.ClassINET, Ttl: 0},
	}

	return rr
}

// build a model/code DNS RR
func (d *Device) ToHINFO() *dns.HINFO {

	rr := &dns.HINFO{
		Hdr: dns.RR_Header{Name: dns.Fqdn(d.Name), Rrtype: dns.TypeHINFO, Class: dns.ClassINET, Ttl: 0},
		Os:  d.Code,
		Cpu: d.Model,
	}

	return rr
}

// build a location DNS RR (use default size etc.)
func (d *Device) ToLOC() *dns.LOC {
	la, lo, a := d.Location()

	rr := &dns.LOC{
		Hdr:       dns.RR_Header{Name: dns.Fqdn(d.Name), Rrtype: dns.TypeLOC, Class: dns.ClassINET, Ttl: 0},
		Size:      cm2size(10000),
		HorizPre:  cm2size(5000),
		VertPre:   cm2size(5000),
		Latitude:  la,
		Longitude: lo,
		Altitude:  a,
	}

	return rr
}

// build a place name DNS RR
func (d *Device) ToTXT() *dns.TXT {

	rr := &dns.TXT{
		Hdr: dns.RR_Header{Name: dns.Fqdn(d.Name), Rrtype: dns.TypeTXT, Class: dns.ClassINET, Ttl: 0},
		Txt: strings.Split(d.Place, " "),
	}

	return rr
}

// dynamically update the device info stored in DNS
func (s *Service) UpdateInfo(zone string, device *Device) error {

	rr := []dns.RR{
		device.ToOPT(),
		device.ToTXT(),
		device.ToHINFO(),
		device.ToLOC(),
	}

	return s.Insert(zone, rr)
}

// dynamically remove the device info stored in DNS (usually prior to an update)
func (s *Service) RemoveInfo(zone string, device *Device) error {

	rr := []dns.RR{
		device.ToOPT(),
		device.ToTXT(),
		device.ToHINFO(),
		device.ToLOC(),
	}

	return s.RemoveRRset(zone, rr)
}

// remove all RR values stored in DNS
func (s *Service) RemoveAll(zone string, device *Device) error {

	rr := &dns.ANY{
		Hdr: dns.RR_Header{Name: dns.Fqdn(device.Name), Rrtype: dns.TypeANY, Class: dns.ClassANY, Ttl: 0},
	}

	return s.RemoveName(zone, []dns.RR{rr})
}

func findPrivateZone(ip net.IP, zone string) string {
	z := zone

	switch {
	case strings.HasPrefix(ip.String(), "10."):
		z = "10.in-addr.arpa."
	case strings.HasPrefix(ip.String(), "172.16."):
		z = "16.172.in-addr.arpa."
	case strings.HasPrefix(ip.String(), "192.168."):
		z = "168.192.in-addr.arpa."
	}

	return z
}

// Dynamically add a set of RR records stored in DNS
func (s *Service) Insert(zone string, rr []dns.RR) error {
	m := new(dns.Msg)

	m.SetUpdate(zone)
	m.SetTsig(dns.Fqdn(s.Key), dns.HmacMD5, 300, time.Now().Unix())
	m.Insert(rr)

	h, err := s.ServerPort()
	if err != nil {
		return err
	}

	c := new(dns.Client)
	c.TsigSecret = map[string]string{dns.Fqdn(s.Key): s.Secret}

	r, _, err := c.Exchange(m, h)
	if err != nil {
		return err
	}

	if r.Rcode != dns.RcodeSuccess {
		return errors.New(fmt.Sprintf("invalid exchange answer"))
	}

	return nil
}

// Dynamically remove a set of RR records stored in DNS
func (s *Service) RemoveRRset(zone string, rr []dns.RR) error {
	m := new(dns.Msg)

	m.SetUpdate(zone)
	m.SetTsig(dns.Fqdn(s.Key), dns.HmacMD5, 300, time.Now().Unix())
	m.RemoveRRset(rr)

	h, err := s.ServerPort()
	if err != nil {
		return err
	}

	c := new(dns.Client)
	c.TsigSecret = map[string]string{dns.Fqdn(s.Key): s.Secret}

	r, _, err := c.Exchange(m, h)
	if err != nil {
		return err
	}

	if r.Rcode != dns.RcodeSuccess {
		return errors.New(fmt.Sprintf("invalid exchange answer"))
	}

	return nil
}

// Dynamically remove a full set of RR records stored in DNS
func (s *Service) RemoveName(zone string, rr []dns.RR) error {
	m := new(dns.Msg)

	m.SetUpdate(zone)
	m.SetTsig(dns.Fqdn(s.Key), dns.HmacMD5, 300, time.Now().Unix())
	m.RemoveName(rr)

	h, err := s.ServerPort()
	if err != nil {
		return err
	}

	c := new(dns.Client)
	c.TsigSecret = map[string]string{dns.Fqdn(s.Key): s.Secret}

	r, _, err := c.Exchange(m, h)
	if err != nil {
		return err
	}

	if r.Rcode != dns.RcodeSuccess {
		return errors.New(fmt.Sprintf("invalid exchange answer"))
	}

	return nil
}

func reverseAddress(ip net.IP) string {
	d := strings.Split(ip.String(), ".")
	for i, j := 0, len(d)-1; i < j; i, j = i+1, j-1 {
		d[i], d[j] = d[j], d[i]
	}
	return strings.Join(d, ".") + ".in-addr.arpa."
}

func (s *Service) UpdateReverse(zone string, ttl uint32, from, to *Device) error {
	for _, r := range from.Reverse {
		if to.HasReverse(r) {
			continue
		}
		z := findPrivateZone(r, zone)
		if z == zone {
			continue
		}

		fmt.Printf("EXTRA REVERSE: %s\n", r.String())
		ptr := &dns.PTR{
			Hdr: dns.RR_Header{Name: reverseAddress(r), Rrtype: dns.TypePTR, Class: dns.ClassINET},
			Ptr: dns.Fqdn(from.Name),
		}
		if err := s.RemoveRRset(z, []dns.RR{ptr}); err != nil {
			return err
		}
	}

	for _, r := range to.Reverse {
		if from.HasReverse(r) {
			continue
		}
		z := findPrivateZone(r, zone)
		if z == zone {
			continue
		}
		fmt.Printf("MISSING REVERSE: %s\n", r.String())
		ptr := &dns.PTR{
			Hdr: dns.RR_Header{Name: reverseAddress(r), Rrtype: dns.TypePTR, Class: dns.ClassINET},
		}
		if err := s.RemoveRRset(z, []dns.RR{ptr}); err != nil {
			return err
		}
		ptr = &dns.PTR{
			Hdr: dns.RR_Header{Name: reverseAddress(r), Rrtype: dns.TypePTR, Class: dns.ClassINET, Ttl: ttl},
			Ptr: dns.Fqdn(to.Name),
		}
		fmt.Println(ptr)
		if err := s.Insert(z, []dns.RR{ptr}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) UpdateAlias(zone string, ttl uint32, from, to *Device) error {
	for _, r := range from.Aliases {
		if to.HasAlias(r) {
			continue
		}
		fmt.Printf("EXTRA ALIAS: %s\n", r)
		cname := &dns.CNAME{
			Hdr:    dns.RR_Header{Name: dns.Fqdn(r), Rrtype: dns.TypeCNAME, Class: dns.ClassINET},
			Target: dns.Fqdn(to.Name),
		}
		fmt.Println(cname)
		if err := s.RemoveRRset(zone, []dns.RR{cname}); err != nil {
			return err
		}
	}

	for _, r := range to.Aliases {
		if from.HasAlias(r) {
			continue
		}
		fmt.Printf("MISSING ALIAS: %s\n", r)
		cname := &dns.CNAME{
			Hdr:    dns.RR_Header{Name: dns.Fqdn(r), Rrtype: dns.TypeCNAME, Class: dns.ClassINET},
			Target: dns.Fqdn(from.Name),
		}
		fmt.Println(cname)
		if err := s.RemoveRRset(zone, []dns.RR{cname}); err != nil {
			return err
		}
		if err := s.Insert(zone, []dns.RR{cname}); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) UpdateMapping(zone string, ttl uint32, from, to *Device) error {
	for m, i := range from.Mapping {
		if to.HasMapping(m, i) {
			continue
		}
		fmt.Printf("EXTRA MAPPING: %s -> %s\n", m, i.String())
		ptr := &dns.PTR{
			Hdr: dns.RR_Header{Name: reverseAddress(i), Rrtype: dns.TypePTR, Class: dns.ClassINET},
			Ptr: dns.Fqdn(m),
		}
		fmt.Println(ptr)
		z := findPrivateZone(i, zone)
		if z == zone {
			continue
		}
		if err := s.RemoveRRset(z, []dns.RR{ptr}); err != nil {
			return err
		}
	}

	for m, i := range to.Mapping {
		if from.HasMapping(m, i) {
			continue
		}
		fmt.Printf("MISSING MAPPING: %s -> %s\n", m, i.String())
		ptr := &dns.PTR{
			Hdr: dns.RR_Header{Name: reverseAddress(i), Rrtype: dns.TypePTR, Class: dns.ClassINET},
			Ptr: dns.Fqdn(m),
		}
		fmt.Println(ptr)
		z := findPrivateZone(i, zone)
		if z == zone {
			continue
		}
		if err := s.RemoveRRset(z, []dns.RR{ptr}); err != nil {
			return err
		}
		if err := s.Insert(z, []dns.RR{ptr}); err != nil {
			return err
		}
	}

	return nil
}

func (s *Service) Update(zone string, ttl uint32, from, to *Device) error {
	if err := s.UpdateReverse(zone, ttl, from, to); err != nil {
		return err
	}
	if err := s.UpdateAlias(zone, ttl, from, to); err != nil {
		return err
	}
	if err := s.UpdateMapping(zone, ttl, from, to); err != nil {
		return err
	}
	return nil
}
