package collector

import "testing"

// The three readings the phone app cannot show today: the sensors, the
// per-device disk counters, and every interface instead of only one.
func TestParseReadsTemperaturesDiskIOAndInterfaces(t *testing.T) {
	raw := "cpu\t100 0 50 800 10 0 0 0\nmem_total\t1000\n" +
		"temp\tcoretemp Package id 0\t42.5\n" +
		"temp\tnvme Composite\t38\n" +
		"diskio\tsda\t2048\t4096\n" +
		"netif\teth0\t100\t200\t1\t2\t3\t4\n" +
		"netif\twg0\t10\t20\t0\t0\t0\t0\n"
	s, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Temperatures) != 2 || s.Temperatures[0].Label != "coretemp Package id 0" || s.Temperatures[0].Celsius != 42.5 {
		t.Fatalf("temperatures: %+v", s.Temperatures)
	}
	if len(s.DiskIO) != 1 || s.DiskIO[0].Device != "sda" || s.DiskIO[0].ReadBytes != 2048 || s.DiskIO[0].WriteBytes != 4096 {
		t.Fatalf("disk io: %+v", s.DiskIO)
	}
	if len(s.Interfaces) != 2 || s.Interfaces[0].Name != "eth0" || s.Interfaces[0].Rx != 100 || s.Interfaces[0].TxDrops != 4 || s.Interfaces[1].Name != "wg0" {
		t.Fatalf("interfaces: %+v", s.Interfaces)
	}
}

// A short line must not create a half-filled entry.
func TestParseSkipsShortDetailLines(t *testing.T) {
	raw := "cpu\t100 0 50 800 10 0 0 0\nmem_total\t1000\ntemp\tonly-a-label\ndiskio\tsda\t1\nnetif\teth0\t1 2\n"
	s, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Temperatures) != 0 || len(s.DiskIO) != 0 || len(s.Interfaces) != 0 {
		t.Fatalf("short lines produced entries: %+v %+v %+v", s.Temperatures, s.DiskIO, s.Interfaces)
	}
}
