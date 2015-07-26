package zone

import (
	"testing"
)

func TestToLocation(t *testing.T) {

	d := Device{Latitude: -41.290438888888886, Longitude: 174.7815961111111, Height: 21}

	if d.ToLocation().String() != "\t0\tIN\tLOC\t41 17 25.580 S 174 46 53.746 E 21m 100m 50m 50m" {
		t.Error("ToLocation")
	}
}
