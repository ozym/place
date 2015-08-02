package zone

import (
	"testing"
)

func TestToOPT(t *testing.T) {

	d := Device{}

	if d.ToOPT().String() != "\n;; OPT PSEUDOSECTION:\n; EDNS: version 0; flags: ; udp: 1" {
		t.Error("ToOPT")
	}
}

func TestToLOC(t *testing.T) {

	d := Device{Latitude: -41.290438888888886, Longitude: 174.7815961111111, Height: 21}

	if d.ToLOC().String() != ".\t0\tIN\tLOC\t41 17 25.580 S 174 46 53.746 E 21m 100m 50m 50m" {
		t.Error("ToLOC")
	}
}

func TestToHINFO(t *testing.T) {

	d := Device{Model: "MODEL", Code: "CODE"}

	if d.ToHINFO().String() != ".\t0\tIN\tHINFO\t\"MODEL\" \"CODE\"" {
		t.Error("ToLOC")
	}
}

func TestToTXT(t *testing.T) {

	d := Device{Place: "PLACE"}

	if d.ToTXT().String() != ".\t0\tIN\tTXT\t\"PLACE\"" {
		t.Error("ToTXT")
	}
}
