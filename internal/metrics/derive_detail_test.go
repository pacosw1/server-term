package metrics

import (
	"testing"
	"time"
)

func TestDeriveComputesDiskAndInterfaceRates(t *testing.T) {
	at := time.Now()
	previous := Sample{At: at, Online: true,
		DiskIO:     []DiskIO{{Device: "sda", ReadBytes: 1000, WriteBytes: 2000}},
		Interfaces: []NetInterface{{Name: "eth0", Rx: 100, Tx: 200}},
	}
	current := Sample{At: at.Add(2 * time.Second), Online: true,
		DiskIO:     []DiskIO{{Device: "sda", ReadBytes: 3000, WriteBytes: 2400}},
		Interfaces: []NetInterface{{Name: "eth0", Rx: 300, Tx: 200}},
	}
	Derive(&previous, &current)
	if current.DiskIO[0].ReadRate != 1000 || current.DiskIO[0].WriteRate != 200 {
		t.Fatalf("disk rates: %+v", current.DiskIO[0])
	}
	if current.Interfaces[0].RxRate != 100 || current.Interfaces[0].TxRate != 0 {
		t.Fatalf("interface rates: %+v", current.Interfaces[0])
	}
}

// A counter that goes backwards means the host restarted the counter. A
// negative rate would read as real traffic, so the rate stays at zero.
func TestDeriveIgnoresCounterReset(t *testing.T) {
	at := time.Now()
	previous := Sample{At: at, Online: true,
		DiskIO:     []DiskIO{{Device: "sda", ReadBytes: 9000}},
		Interfaces: []NetInterface{{Name: "eth0", Rx: 9000}},
	}
	current := Sample{At: at.Add(time.Second), Online: true,
		DiskIO:     []DiskIO{{Device: "sda", ReadBytes: 10}},
		Interfaces: []NetInterface{{Name: "eth0", Rx: 10}},
	}
	Derive(&previous, &current)
	if current.DiskIO[0].ReadRate != 0 || current.Interfaces[0].RxRate != 0 {
		t.Fatalf("reset produced a rate: %+v %+v", current.DiskIO[0], current.Interfaces[0])
	}
}

// A device that appears between two samples has no earlier reading, so it
// gets no rate rather than a rate computed against zero.
func TestDeriveSkipsNewDevice(t *testing.T) {
	at := time.Now()
	previous := Sample{At: at, Online: true}
	current := Sample{At: at.Add(time.Second), Online: true,
		DiskIO:     []DiskIO{{Device: "sdb", ReadBytes: 500}},
		Interfaces: []NetInterface{{Name: "wg0", Rx: 500}},
	}
	Derive(&previous, &current)
	if current.DiskIO[0].ReadRate != 0 || current.Interfaces[0].RxRate != 0 {
		t.Fatalf("new device produced a rate: %+v %+v", current.DiskIO[0], current.Interfaces[0])
	}
}
